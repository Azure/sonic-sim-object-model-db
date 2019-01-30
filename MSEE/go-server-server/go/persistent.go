package mseeserver

import (
    "arpthrift"
    "errors"
    "fmt"
    "git.apache.org/thrift.git/lib/go/thrift"
    "github.com/go-redis/redis"
    "log"
    "mseethrift"
    "net"
    "strconv"
    "strings"
    "swsscommon"
    "time"
    "bytes"
    "github.com/satori/go.uuid"
    "sync"
)

const ServerAPIVersion string = "1.0.0"
var ServerResetGuid string
var ServerResetTime string

var redisDB *redis.Client
var mseeClient *msee.MSEEClient
var arpClient *arp.ArpResponderClient
var swssDB swsscommon.DBConnector
var swss_conf_DB swsscommon.DBConnector
var trustedertCommonNames []string

var portIDMap map[string]int
var portNameMap map[int]string
var portCounterIDMap map[string]string

var vnetGuidMap map[string]uint32
var vnetGuidIdUsed []bool
var nextGuidId uint32
var GuidMapMutex sync.RWMutex

var configSnapshot *ServerSnapshotModel

const REDIS_SOCK string = "/var/run/redis/redis.sock"

const APPL_DB int = 0
const COUNTER_DB int = 2
const CONFIG_DB int = 4

// TODO:
// DB 4 is reserved for config DB, we can not simply flush it any more
// when we reset the server. It will affect other applications on Sonic.
// Let use use DB 7 for caching now and move the info in cache to config DB
// with new schema and delete only keys for this server in Reset.
const APPL_CACHE_DB int = 7

const SWSS_TIMEOUT uint = 0

const DPDK_vlan_mtu string = "9100"
const DPDK_vlan_sw_ip   string = "1.1.1.1"
const DPDK_vlan_sw_len  string = "31"
const DPDK_vlan_dpdk_ip string = "1.1.1.2"

// DB Table names
const VXLAN_TUNNEL_TB   string = "VXLAN_TUNNEL"
const VNET_TB           string = "VNET"
const VLAN_TB           string = "VLAN"
const VLAN_INTF_TB      string = "VLAN_INTERFACE"
const VLAN_MEMB_TB      string = "VLAN_MEMBER"
const VLAN_NEIGH_TB     string = "NEIGH"
const ROUTE_TUN_TB      string = "VNET_ROUTE_TUNNEL_TABLE"
const LOCAL_ROUTE_TB    string = "VNET_ROUTE_TABLE"

// DB Helper constants
const VNET_NAME_PREF  string = "Vnet"
const VLAN_NAME_PREF  string = "Vlan"

type db_ops struct {
   separator string
   swss_db  swsscommon.DBConnector
   db_num   int
}

var app_db_ops db_ops
var conf_db_ops db_ops

func Initialise() {
    //DPDKThriftConnect()
    //ARPThriftConnect()
    DBConnect(*ResetFlag)
    //AddPortsToVLANs()
    GetPortsFromCounterDB()
    //AddDPDKPort()
    InitialiseVariables()
}

func InitialiseVariables() {
    trustedertCommonNames = strings.Split(*ClientCertCommonNameFlag, ",")
    //TODO: need to reload configSnapshot when RESET flag is not set after we have config DB
    configSnapshot = &ServerSnapshotModel{}
    configSnapshot.VrfMap = make(map[int]VrfSnapshotModel)

    var err error
    ServerResetGuid, ServerResetTime, err = CacheGetConfigResetInfo()

    if err == redis.Nil {
        loc, _ := time.LoadLocation("UTC")
        now := time.Now().In(loc)
        ServerResetTime = fmt.Sprintf("%v", now)

        newuuid, _ := uuid.NewV4()
        ServerResetGuid = newuuid.String()

        err = CacheSetConfigResetInfo(ServerResetGuid, ServerResetTime)
        if err != nil {
            log.Fatalf("error: could not save reset info to DB, error: %+v", err)
        }
        log.Printf("info: set config reset Guid and Time to %v, %v", ServerResetGuid, ServerResetTime)
    } else if err == nil {
        log.Printf("info: find config reset Guid and Time in DB as %v, %v", ServerResetGuid, ServerResetTime)
    } else {
        log.Fatalf("error: could not retrieve server reset info from DB, error: %+v", err)
    }
    genVnetGuidMap()
}


func genVnetGuidMap() {
    vnetGuidMap = make(map[string]uint32)
    var max_ind, i uint32 = 0, 0
    db := &conf_db_ops
    kv, err := GetKVsMulti(db.db_num, generateDBTableKey(db.separator, VNET_TB, "*"))

    if (err != nil) || (len(kv) == 0) {
        log.Printf("info: No VNET tables found to gen Vnet Guid Map, default init, err: %d len kv: %d", err, len(kv))
        vnetGuidIdUsed = make([]bool, 0, 30) /* 30 HSM/rack? */
        nextGuidId = 1
        return
    }

    for k, v := range kv {
         log.Printf("info: TABLE: %s TABLE_KVs: %#v", k, v)
         tb_key_len := len(generateDBTableKey(db.separator, VNET_TB, VNET_NAME_PREF))
         vnet_id_str := k[tb_key_len:]
         vnet_id_64, err_c := strconv.ParseUint(vnet_id_str, 10, 32)
         if (err_c != nil) || (vnet_id_64 == 0) {
             log.Printf("error: Found non integer vnet_id %s", vnet_id_str)
         }
         vnet_id := uint32(vnet_id_64)
         if v["guid"] == "" {
             log.Printf("error: Found nil guid %s %s", k, vnet_id_str)
             continue
         }
         log.Printf("info: storing vnet-guid: %s, vnet_id %d", v["guid"], vnet_id)
         vnetGuidMap[v["guid"]] = vnet_id
         if vnet_id > max_ind {
             max_ind = vnet_id
         }
    }
    vnetGuidIdUsed = make([]bool, max_ind, max_ind)
    for _, v := range vnetGuidMap {
        vnetGuidIdUsed[v - 1] = true
    }
    for i = 0; i < max_ind; i++ {
        if !vnetGuidIdUsed[i] {
             break
        }
    }
    nextGuidId = i + 1
}

func DBConnect(reset bool) {
    if *RunApiAsLocalTestDocker {
        redisDB = redis.NewClient(&redis.Options{
            Addr:     "localhost:6379",
            Password: "",
        })
    } else {
        redisDB = redis.NewClient(&redis.Options{
            Network:  "unix",
            Addr:     REDIS_SOCK,
            Password: "",
        })
    }

    if reset {
        pipe := redisDB.TxPipeline()
        pipe.Select(APPL_CACHE_DB)
        pipe.FlushDB()
        _, err := pipe.Exec()

        if err != nil {
            log.Fatalf("error: could not reset Redis cache DB, error: %+v", err)
        }
    }

    cache_status := "cache loaded"
    if (reset) {
        cache_status = "cache cleaned"
    }

    log.Printf("info: Redis connection established (%+v), %s", redisDB, cache_status)

    if *RunApiAsLocalTestDocker {
	     swssDB = swsscommon.NewDBConnector(APPL_DB, "localhost", 6379, SWSS_TIMEOUT)
	     swss_conf_DB = swsscommon.NewDBConnector(CONFIG_DB, "localhost", 6379, SWSS_TIMEOUT)
    } else {
        swssDB = swsscommon.NewDBConnector2(APPL_DB, REDIS_SOCK, SWSS_TIMEOUT)
	     swss_conf_DB = swsscommon.NewDBConnector2(CONFIG_DB, REDIS_SOCK, SWSS_TIMEOUT)
    }
    app_db_ops = db_ops{separator: ":", swss_db: swssDB, db_num: APPL_DB}
    conf_db_ops = db_ops{separator: "|", swss_db: swss_conf_DB, db_num: CONFIG_DB}
}

func DPDKThriftConnect() {
    transport, err := thrift.NewTSocket(*ThriftHostFlag)

    if err != nil {
        log.Fatalf("error: opening socket: %s", err)
    }

    if err := transport.Open(); err != nil {
        log.Fatalf("error: opening transport %s", err)
    }

    protocol := thrift.NewTBinaryProtocolTransport(transport)

    mseeClient = msee.NewMSEEClientProtocol(transport, protocol, protocol)

    macAddr, err := net.ParseMAC(*SWMacAddrFlag)
    if err != nil {
        log.Fatalf("error: invalid address: %s with %s", *SWMacAddrFlag, err)
    }

    macint64 := msee.MseeMacT(MacToInt64(macAddr))

    lo4 := GetThriftIPAddress(net.ParseIP(*LoAddr4Flag))
    lo6 := GetThriftIPAddress(net.ParseIP(*LoAddr6Flag))

    ports := strings.Split(*PortsFlag, ",")
    nb_of_ports := len(ports)

    ret, err := mseeClient.InitDpdkPort(msee.MseePortCountT(nb_of_ports), macint64, *lo4.IP.Ip4, lo6.IP.Ip6)
    log.Printf("trace: thrift: InitDpdkPort(%v, %v, %v, %v) = (%v, %v)", nb_of_ports, macint64, lo4, lo6, ret, err)

    if err != nil || ret != msee.ResultT_OK {
        log.Fatalf("error: thrift: InitDpdkPort(%v, %v, %v, %v) = (%v, %v)", nb_of_ports, macint64, lo4, lo6, ret, err)
    }
}

func ARPThriftConnect() {
    transport, err := thrift.NewTSocket(*ARPHostFlag)

    if err != nil {
        log.Printf("warning: opening socket: %s", err)
    }

    if err := transport.Open(); err != nil {
        log.Printf("warning: opening transport %s", err)
    }

    protocol := thrift.NewTBinaryProtocolTransport(transport)

    arpClient = arp.NewArpResponderClientProtocol(transport, protocol, protocol)
}

func AddPortsToVLANs() {
    // Add ports to VLAN
    phyPorts, err := GetPorts("Ethernet")
    if err != nil {
        log.Fatalf("error: could not get list of ethernet ports")
    }

    pt := swsscommon.NewProducerStateTable(swssDB, "VLAN_TABLE")
    defer pt.Delete()

    vpt := swsscommon.NewProducerStateTable(swssDB, "VLAN_MEMBER_TABLE")
    defer vpt.Delete()

    ports := strings.Split(*PortsFlag, ",")

    i := 0

    portIDMap = make(map[string]int)
    portNameMap = make(map[int]string)

    for _, port := range ports {
        valid := false

        for _, phyPort := range phyPorts {
            if phyPort == port {
                valid = true
                break
            }
        }

        if !valid {
            log.Fatalf("error: no such port %s", port)
        }

        portIDMap[port] = i
        portNameMap[i] = port
        i++

        vlan, err := PortToVlanID(port)
        if err != nil {
            log.Fatalf("error: invalid port format %s", port)
        }

        kv, err := SwssGetKVs(fmt.Sprintf("VLAN_TABLE:Vlan%d", vlan))
        if err != nil {
            log.Fatalf("error: %s", err)
        }

        // Create if not found
        // Setup untagged vlans for each port so that they are isolated
        if kv == nil {
            pt.Set(fmt.Sprintf("Vlan%d", vlan), map[string]string{
                "admin_status": "up",
                "oper_status": "up",
            }, "SET", "")

            vpt.Set(fmt.Sprintf("Vlan%d:%s", vlan, port), map[string]string{
                "tagging_mode": "untagged",
            }, "SET", "")
        }
    }
}

func GetPortsFromCounterDB() {
    portCounterIDMap = make(map[string]string)

    pipe := redisDB.TxPipeline()
    pipe.Select(COUNTER_DB)
    kvRes := pipe.HGetAll("COUNTERS_PORT_NAME_MAP")
    _, err := pipe.Exec()
    if err != nil || kvRes.Err() != nil {
        log.Print("warning: could not find any COUNTERS_PORT_NAME_MAP in CounterDB")
        return
    }

    for k, v := range kvRes.Val() {
        portCounterIDMap[k] = v
    }

    return
}

func AddDPDKPort() {
    // Add swss configuration for DPDK port. Required by RedirectEgr ACL rule
    PublishSWSS_SET("VLAN_TABLE", fmt.Sprintf("Vlan%d", *DpdkVlanFlag), map[string]string{
        "admin_status": "up",
        "oper_status": "up",
        "mtu": DPDK_vlan_mtu,
    })

    PublishSWSS_SET("VLAN_MEMBER_TABLE", fmt.Sprintf("Vlan%d:%s", *DpdkVlanFlag, *DpdkPortFlag), map[string]string{
        "tagging_mode": "untagged",
    })

    PublishSWSS_SET("INTF_TABLE", fmt.Sprintf("Vlan%d:%s/%s", *DpdkVlanFlag, DPDK_vlan_sw_ip, DPDK_vlan_sw_len), map[string]string{
        "scope": "global",
        "family": "IPv4",
    })

    macAddr, err := net.ParseMAC(*DPDKMacAddrFlag)
    if err != nil {
        log.Fatalf("error: invalid address: %s with %s", *DPDKMacAddrFlag, err)
    }

    s_macAddr := macAddr.String()
    s_alt_macAddr := strings.Replace(s_macAddr, ":", "-", -1)
    PublishSWSS_SET("FDB_TABLE", fmt.Sprintf("Vlan%d:%s", *DpdkVlanFlag, s_alt_macAddr), map[string]string{
        "port": *DpdkPortFlag,
        "type": "static",
    })

    PublishSWSS_SET("NEIGH_TABLE", fmt.Sprintf("Vlan%d:%s", *DpdkVlanFlag, DPDK_vlan_dpdk_ip), map[string]string{
        "neigh": s_macAddr,
        "family": "IPv4",
    })
}

func PublishSWSS(op string, table string, key string, attrs map[string]string) {
    kv, err := SwssGetKVs(fmt.Sprintf(fmt.Sprintf("%s:%s", table, key)))
    if err != nil {
        log.Fatalf("error: %s", err)
    }

    obj := swsscommon.NewProducerStateTable(swssDB, table)
    defer obj.Delete()

    if kv == nil {
        obj.Set(key, attrs, op, "")
    }
}

func PublishSWSS_SET(table string, key string, attrs map[string]string) {
    PublishSWSS("SET", table, key, attrs)
}

func PublishSWSS_DEL(table string, key string) {
    PublishSWSS("DEL", table, key, map[string]string{})
}

func CacheGetPortAddr(port string) (IPaddr string, MACAddress string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    addrCmd_ip := pipe.HGet("PORT_ADDR_MAP", port)
    addrCmd_mac := pipe.HGet("PORT_MAC_MAP", port)
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    if addrCmd_ip.Err() != nil {
        err = addrCmd_ip.Err()
        return
    }

    if addrCmd_mac.Err() != nil {
        err = addrCmd_mac.Err()
        return
    }

    return addrCmd_ip.Val(), addrCmd_mac.Val(), nil
}

func CacheSetPortAddr(port string, IPaddr string, MACAddress string) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    setCmd_ip := pipe.HSet("PORT_ADDR_MAP", port, IPaddr)
    setCmd_mac := pipe.HSet("PORT_MAC_MAP", port, MACAddress)
    _, err := pipe.Exec()
    if err != nil {
        return err
    }

    if setCmd_ip.Err() != nil {
        return setCmd_ip.Err()
    }

    if setCmd_mac.Err() != nil {
        return setCmd_mac.Err()
    }

    return nil
}

func CacheDeletePortAddr(port string) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    delCmd_ip := pipe.HDel("PORT_ADDR_MAP", port)
    delCmd_mac := pipe.HDel("PORT_MAC_MAP", port)
    _, err := pipe.Exec()
    if err != nil {
        return err
    }

    if delCmd_ip.Err() != nil {
        return delCmd_ip.Err()
    }

    if delCmd_mac.Err() != nil {
        return delCmd_mac.Err()
    }

    return nil
}

func CacheGetVrfName(vrfID int) (vrfName string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    vrfNameCmd := pipe.HGet("VRFID_VRFNAME_MAP", strconv.Itoa(vrfID))
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    vrfName = vrfNameCmd.Val()
    return
}

func CacheSetVrfName(vrfID int, vrfName string) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    ret := pipe.Eval(`
-- ARGV[1] : vrf_id
-- ARGV[2] : vrf_name
-- returns : true is successful, false if vrf_name already exists

local vrf_id = redis.call('hget', 'VRFNAME_VRFID_MAP', ARGV[2])
if vrf_id then
    return false
else
    local vrf_name = redis.call('hget', 'VRFID_VRFNAME_MAP', ARGV[1])
    if vrf_name then
        -- Each vrf_id may only have one name, but it may be changed
        redis.call('hdel', 'VRFNAME_VRFID_MAP', vrf_name)
    end

    redis.call('hset', 'VRFNAME_VRFID_MAP', ARGV[2], ARGV[1])
    redis.call('hset', 'VRFID_VRFNAME_MAP', ARGV[1], ARGV[2])

    return true
end
`, []string{}, vrfID, vrfName)
    _, err := pipe.Exec()
    if err != nil {
        return err
    }

    if ret.Val() == nil {
        return errors.New("Duplicate vrf_name")
    }

    return nil
}

func CacheDeleteVrfID(vrfID int) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    ret := pipe.Eval(`
-- ARGV[1] : vrf_id
-- returns : vrf_name unmapped is successful, nil otherwise

local vrf_name = redis.call('hget', 'VRFID_VRFNAME_MAP', ARGV[1])
if vrf_name then
    if vrf_name then
        -- Each vrf_id may only have one name, but it may be changed
    end

    redis.call('hdel', 'VRFNAME_VRFID_MAP', vrf_name)
    redis.call('hdel', 'VRFID_VRFNAME_MAP', ARGV[1])

    return vrf_name
else
    return nil
end
`, []string{}, vrfID)
    _, err := pipe.Exec()
    if err != nil {
        return err
    }

    if ret.Err() == redis.Nil {
        return errors.New("vrf_id not found")
    }

    return nil
}

func CacheGetVrfID(vrfName string) (vrfID int, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    vrfIDCmd := pipe.HGet("VRFNAME_VRFID_MAP", vrfName)
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    vrfID, err = strconv.Atoi(vrfIDCmd.Val())
    if err != nil {
        return
    }

    return
}

func SwssGetVrfPorts(vrf string) (ports []*arp.ReqTupleT, err error) {
    kv, err := SwssGetKVsMulti("QINQ_TABLE:*")

    if err != nil {
        return
    }

    if len(kv) == 0 {
        return
    }

    for k, v := range kv {
        table := k[len("QINQ_TABLE:"):]
        if v["vrf_id"] == vrf {
            tableparts := strings.Split(table, ":")
            port := tableparts[0]
            stag, _ := strconv.Atoi(tableparts[1])
            ctag, _ := strconv.Atoi( tableparts[2])
            ports = append(ports, &arp.ReqTupleT{
                IfaceName: port,
                Stag: arp.VlanTagT(stag),
                Ctag: arp.VlanTagT(ctag),
            })
        }
    }

    return
}

func CacheClearVrf(vrf string) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    _ = pipe.Del("VRF_QINQ:"+vrf)
    _, err := pipe.Exec()
    return err
}

func SwssGetKVs(key string) (kv map[string]string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_DB)
    kvRes := pipe.HGetAll(key)
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    kv = kvRes.Val()
    if len(kv) == 0 {
        kv = nil
    }

    return
}

func ConfigDBGetKVs(key string) (kv map[string]string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(CONFIG_DB)
    kvRes := pipe.HGetAll(key)
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    kv = kvRes.Val()
    if len(kv) == 0 {
        kv = nil
    }

    return
}

func ConfigDBDelKey(key string) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(CONFIG_DB)
    _ = pipe.Del(key)
    _, err := pipe.Exec()
    return err
}

func ConfigDBSetKey(key string, kv map[string]interface{}) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(CONFIG_DB)
    _ = pipe.HMSet(key, kv)
    _, err := pipe.Exec()
    return err
}

func SwssGetKVsMulti(pattern string) (kv map[string]map[string]string, err error) {
    var cursor uint64

    kv = make(map[string]map[string]string)

    for {
        pipe := redisDB.TxPipeline()
        pipe.Select(APPL_DB)
        ret := pipe.Scan(cursor, pattern, 1)

        _, err = pipe.Exec()
        if err != nil {
            return
        }

        var keys []string
        keys, cursor = ret.Val()

        for _, key := range keys {
            kv[key], err = SwssGetKVs(key)
            if err != nil {
                return
            }
        }

        if cursor == 0 {
            break
        }
    }

    return
}


func GetKVs(DB int, key string) (kv map[string]string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(DB)
    kvRes := pipe.HGetAll(key)
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    kv = kvRes.Val()
    if len(kv) == 0 {
        kv = nil
    }

    return
}

func GetKVsMulti(DB int, pattern string) (kv map[string]map[string]string, err error) {
    var cursor uint64

    kv = make(map[string]map[string]string)

    for {
        pipe := redisDB.TxPipeline()
        pipe.Select(DB)
        ret := pipe.Scan(cursor, pattern, 1)

        _, err = pipe.Exec()
        if err != nil {
            return
        }

        var keys []string
        keys, cursor = ret.Val()

        for _, key := range keys {
            kv[key], err = GetKVs(DB, key)
            if err != nil {
                return
            }
        }

        if cursor == 0 {
            break
        }
    }

    return
}

func SwssGetVrouterRoutes(vnet_id_str string, vnidMatch int, ipFilter string) (routes []RouteModel, err error) {
    db := &app_db_ops
    var pattern string
    // TODO: Keep only else statement code in production
    if *RunApiAsLocalTestDocker {
        pattern = generateDBTableKey(db.separator, "_"+ROUTE_TUN_TB, vnet_id_str, ipFilter)
    } else {
        pattern = generateDBTableKey(db.separator, ROUTE_TUN_TB, vnet_id_str, ipFilter)
    }
    routes = []RouteModel{}

    kv, err := GetKVsMulti(db.db_num,pattern)
    if err != nil {
        return
    }

    for k, kvp := range kv {
        ipprefix := strings.Split(k, db.separator)[2]

        routeModel := RouteModel{
            IPPrefix:    ipprefix,
            NextHop:     kvp["endpoint"],
        }

        if vnid, ok := kvp["vni"]; ok {
            routeModel.Vnid, _ = strconv.Atoi(vnid)
        }

        if vnidMatch >= 0 {
            if vnidMatch != routeModel.Vnid {
                continue
            }
        }

        if mac, ok := kvp["mac_address"]; ok {
            routeModel.MACAddress = mac
        }

        routes = append(routes, routeModel)
    }

    return
}

func CounterGetKVs(key string) (kv map[string]string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(COUNTER_DB)
    kvRes := pipe.HGetAll(key)
    _, err = pipe.Exec()
    if err != nil {
        return
    }

    kv = kvRes.Val()
    if len(kv) == 0 {
        kv = nil
    }

    return
}

func CacheGetConfigResetInfo() (GUID string, time string, err error) {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    getCmd_guid := pipe.HGet("RESET_INFO", "GUID")
    getCmd_time := pipe.HGet("RESET_INFO", "time")
    _, err = pipe.Exec()

    if err != nil {
        return
    }

    if getCmd_guid.Err() != nil {
        err = getCmd_guid.Err()
        return
    }

    if getCmd_time.Err() != nil {
        err = getCmd_time.Err()
        return
    }

    return getCmd_guid.Val(), getCmd_time.Val(), nil
}

func CacheSetConfigResetInfo(GUID string, time string) error {
    pipe := redisDB.TxPipeline()
    pipe.Select(APPL_CACHE_DB)
    setCmd_guid := pipe.HSet("RESET_INFO", "GUID", GUID)
    setCmd_time := pipe.HSet("RESET_INFO", "time", time)
    _, err := pipe.Exec()
    if err != nil {
        return err
    }

    if setCmd_guid.Err() != nil {
        return setCmd_guid.Err()
    }

    if setCmd_time.Err() != nil {
        return setCmd_time.Err()
    }

    return nil
}

func CacheGetVnetGuidId(GUID string) (val uint32) {
    GuidMapMutex.RLock()
    defer GuidMapMutex.RUnlock()
    val = vnetGuidMap[GUID]
    return
}

func CacheGenAndSetVnetGuidId(GUID string) (val uint32) {
    GuidMapMutex.Lock()
    defer GuidMapMutex.Unlock()
    vnetGuidMap[GUID] = nextGuidId
    val = nextGuidId
    if nextGuidId == (uint32)(len(vnetGuidIdUsed) + 1) {
        vnetGuidIdUsed = append(vnetGuidIdUsed, true)
        nextGuidId++
    } else {
        var i uint32
        vnetGuidIdUsed[nextGuidId - 1] = true
        for i = nextGuidId; i < (uint32) (len(vnetGuidIdUsed)); i++ {
            if !vnetGuidIdUsed[i] {
                break
            }
        }
        nextGuidId = i + 1
    }
    return
}

func CacheDeleteVnetGuidId(GUID string) {
    GuidMapMutex.Lock()
    defer GuidMapMutex.Unlock()
    i := vnetGuidMap[GUID]
    vnetGuidIdUsed[i - 1] = false
    if i < nextGuidId {
         nextGuidId = i
    }
    delete(vnetGuidMap, GUID)
}

func generateDBTableKey(separator string, vars ...string) (string) {
     var buf bytes.Buffer
     for i := 0; i < len(vars) ; i++ {
         buf.WriteString(vars[i])
         if i != (len(vars) - 1) {
               buf.WriteString(separator)
         }
    }
    return buf.String()
}
