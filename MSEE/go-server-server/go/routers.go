package mseeserver

import (
    "net/http"
    "fmt"
    "github.com/gorilla/mux"
)

type Route struct {
    Name        string
    Method      string
    Pattern     string
    HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
    router := mux.NewRouter().StrictSlash(true)
    for _, route := range routes {
        handler := Logger(route.HandlerFunc, route.Name)

        router.
            Methods(route.Method).
            Path(route.Pattern).
            Name(route.Name).
            Handler(handler)
    }

    return router
}

func Index(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Sonic MSEE Restful API v1!")
}

var routes = Routes{
    Route{
        "Index",
        "GET",
        "/v1/",
        Index,
    },

    Route{
        "StateHeartbeatGet",
        "GET",
        "/v1/state/heartbeat",
        StateHeartbeatGet,
    },

    Route{
        "Config",
        "GET",
        "/v1/config",
        ConfigGet,
    },

    Route{
        "ConfigInterfacePortPortDelete",
        "DELETE",
        "/v1/config/interface/port/{port}",
        ConfigInterfacePortPortDelete,
    },

    Route{
        "ConfigInterfacePortPortGet",
        "GET",
        "/v1/config/interface/port/{port}",
        ConfigInterfacePortPortGet,
    },

    Route{
        "ConfigInterfacePortPortPut",
        "PUT",
        "/v1/config/interface/port/{port}",
        ConfigInterfacePortPortPut,
    },

    Route{
        "ConfigInterfaceVlanDelete",
        "DELETE",
        "/v1/config/interface/vlan/{vlan_id}",
        ConfigInterfaceVlanDelete,
    },

    Route{
        "ConfigInterfaceVlanGet",
        "GET",
        "/v1/config/interface/vlan/{vlan_id}",
        ConfigInterfaceVlanGet,
    },

    Route{
        "ConfigInterfaceVlanPost",
        "POST",
        "/v1/config/interface/vlan/{vlan_id}",
        ConfigInterfaceVlanPost,
    },

    Route{
        "ConfigInterfaceVlanMemberDelete",
        "DELETE",
        "/v1/config/interface/vlan/{vlan_id}/member/{if_name}",
        ConfigInterfaceVlanMemberDelete,
    },

    Route{
        "ConfigInterfaceVlanMemberGet",
        "GET",
        "/v1/config/interface/vlan/{vlan_id}/member/{if_name}",
        ConfigInterfaceVlanMemberGet,
    },

    Route{
        "ConfigInterfaceVlanMemberPost",
        "POST",
        "/v1/config/interface/vlan/{vlan_id}/member/{if_name}",
        ConfigInterfaceVlanMemberPost,
    },

    Route{
        "ConfigInterfaceVlanNeighborDelete",
        "DELETE",
        "/v1/config/interface/vlan/{vlan_id}/neighbor/{ip_addr}",
        ConfigInterfaceVlanNeighborDelete,
    },

    Route{
        "ConfigInterfaceVlanNeighborGet",
        "GET",
        "/v1/config/interface/vlan/{vlan_id}/neighbor/{ip_addr}",
        ConfigInterfaceVlanNeighborGet,
    },

    Route{
        "ConfigInterfaceVlanNeighborPost",
        "POST",
        "/v1/config/interface/vlan/{vlan_id}/neighbor/{ip_addr}",
        ConfigInterfaceVlanNeighborPost,
    },

    Route{
        "ConfigInterfaceQinqPortDelete",
        "DELETE",
        "/v1/config/interface/qinq/{port}",
        ConfigInterfaceQinqPortDelete,
    },

    Route{
        "ConfigInterfaceQinqPortGet",
        "GET",
        "/v1/config/interface/qinq/{port}",
        ConfigInterfaceQinqPortGet,
    },

    Route{
        "ConfigInterfaceQinqPortStagCtagDelete",
        "DELETE",
        "/v1/config/interface/qinq/{port}/{stag}/{ctag}",
        ConfigInterfaceQinqPortStagCtagDelete,
    },

    Route{
        "ConfigInterfaceQinqPortStagCtagGet",
        "GET",
        "/v1/config/interface/qinq/{port}/{stag}/{ctag}",
        ConfigInterfaceQinqPortStagCtagGet,
    },

    Route{
        "ConfigInterfaceQinqPortStagCtagPut",
        "PUT",
        "/v1/config/interface/qinq/{port}/{stag}/{ctag}",
        ConfigInterfaceQinqPortStagCtagPut,
    },

    Route{
        "ConfigTunnelDecapTunnelTypeDelete",
        "DELETE",
        "/v1/config/tunnel/decap/{tunnel_type}",
        ConfigTunnelDecapTunnelTypeDelete,
    },

    Route{
        "ConfigTunnelDecapTunnelTypeGet",
        "GET",
        "/v1/config/tunnel/decap/{tunnel_type}",
        ConfigTunnelDecapTunnelTypeGet,
    },

    Route{
        "ConfigTunnelDecapTunnelTypePost",
        "POST",
        "/v1/config/tunnel/decap/{tunnel_type}",
        ConfigTunnelDecapTunnelTypePost,
    },

    Route{
        "ConfigTunnelEncapVxlanGet",
        "GET",
        "/v1/config/tunnel/encap/vxlan",
        ConfigTunnelEncapVxlanGet,
    },

    Route{
        "ConfigTunnelEncapVxlanVnidDelete",
        "DELETE",
        "/v1/config/tunnel/encap/vxlan/{vnid}",
        ConfigTunnelEncapVxlanVnidDelete,
    },

    Route{
        "ConfigTunnelEncapVxlanVnidGet",
        "GET",
        "/v1/config/tunnel/encap/vxlan/{vnid}",
        ConfigTunnelEncapVxlanVnidGet,
    },

    Route{
        "ConfigTunnelEncapVxlanVnidPost",
        "POST",
        "/v1/config/tunnel/encap/vxlan/{vnid}",
        ConfigTunnelEncapVxlanVnidPost,
    },

    Route{
        "ConfigVrouterGet",
        "GET",
        "/v1/config/vrouter",
        ConfigVrouterGet,
    },

    Route{
        "ConfigVrouterVrfIdDelete",
        "DELETE",
        "/v1/config/vrouter/{vnet_name}",
        ConfigVrouterVrfIdDelete,
    },

    Route{
        "ConfigVrouterVrfIdGet",
        "GET",
        "/v1/config/vrouter/{vnet_name}",
        ConfigVrouterVrfIdGet,
    },

    Route{
        "ConfigVrouterVrfIdPost",
        "POST",
        "/v1/config/vrouter/{vnet_name}",
        ConfigVrouterVrfIdPost,
    },

    Route{
        "ConfigVrouterVrfIdRoutesDelete",
        "DELETE",
        "/v1/config/vrouter/{vnet_name}/routes",
        ConfigVrouterVrfIdRoutesDelete,
    },

    Route{
        "ConfigVrouterVrfIdRoutesGet",
        "GET",
        "/v1/config/vrouter/{vnet_name}/routes",
        ConfigVrouterVrfIdRoutesGet,
    },

    Route{
        "ConfigVrouterVrfIdRoutesPatch",
        "PATCH",
        "/v1/config/vrouter/{vnet_name}/routes",
        ConfigVrouterVrfIdRoutesPatch,
    },

    Route{
        "StateCounterGet",
        "GET",
        "/v1/state/counter",
        StateCounterGet,
    },

    Route{
        "StateInterfacePortGet",
        "GET",
        "/v1/state/interface/{port}",
        StateInterfacePortGet,
    },

    Route{
        "StateInterfaceGet",
        "GET",
        "/v1/state/interface",
        StateInterfaceGet,
    },

    Route{
        "StateCounterGroupGet",
        "GET",
        "/v1/state/counter/{group}",
        StateCounterGroupGet,
    },

    Route{
        "StateHistogramGet",
        "GET",
        "/v1/state/histogram",
        StateHistogramGet,
    },

    Route{
        "StateStatisticsGet",
        "GET",
        "/v1/state/statistics",
        StateStatisticsGet,
    },

    Route{
        "StateStatisticsGroupGet",
        "GET",
        "/v1/state/statistics/{group}",
        StateStatisticsGroupGet,
    },

    // Required to run Unit tests
    Route{
        "InMemConfigRestart",
        "POST",
        "/v1/config/restartdb",
        InMemConfigRestart,
    },
}
