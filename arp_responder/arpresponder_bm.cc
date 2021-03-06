#include <netinet/in.h>
#include <netinet/if_ether.h>
#include <linux/if.h>
#include <linux/filter.h>
#include <vector>
#include <unordered_map>
#include "fmt.h"
#include "log.h"
#include "intf.h"
#include "poller.h"
#include "arp.h"
#include "arpresponder_bm.h"


ARPResponder::ARPResponder()
{
    poller = new Poller();

    LOG_INFO("Starting arpresponder");
}

ARPResponder::~ARPResponder()
{
    LOG_INFO("Stopping arpresonder");

    for (auto intf_pair: interfaces)
    {
        poller->del_fd(intf_pair.first);
        intf_pair.second->close();
        delete intf_pair.second;
    }

    delete poller;
}

void ARPResponder::run()
{
    LOG_INFO("Starting main loop of arpresponder");

    std::vector<int> fds(MAX_NUM_OF_INTERFACES);
    while(true)
    {
        fds.clear();
        poller->poll(fds);
        for(auto fd: fds)
            process(fd);
    }
}

void ARPResponder::process(const int fd)
{
    RawArp arp;
    Interface* iface = interfaces[fd];
    auto ret = iface->recv(arp.get_packet(), arp.size());
    if (ret == -1) return;

    if (!arp.is_valid()) return;

    if (arp.get_type() == ARPOP_REQUEST)
    {
        LOG_DEBUG("Got arp request on %s ip=%s", iface->get_name().c_str(), s_ip(arp.get_dst_ip()).c_str());

        arp.make_reply_from_request(*iface);
        (void)iface->send(arp.get_packet(), arp.size());
    }
}

void ARPResponder::add_interface(const std::string& iface_name)
{
    struct sock_filter arp_code_bpf[] = {
        { 0x28, 0, 0, 0x0000000c }, // (000) ldh      [12]
        { 0x15, 0, 1, 0x00000806 }, // (001) jeq      #0x806           jt 2    jf 3
        { 0x6,  0, 0, 0x00040000 }, // (002) ret      #262144
        { 0x6,  0, 0, 0x00000000 }, // (003) ret      #0
    };
// generated by tcpdump -ieth0 -d arp

#define ARRAY_SIZE(x) (sizeof(x) / sizeof((x)[0]))

    struct sock_fprog arp_bpf = {
        .len = ARRAY_SIZE(arp_code_bpf),
        .filter = arp_code_bpf
    };

    LOG_DEBUG("Adding interface %s", iface_name.c_str());
    Interface* iface = new Interface(iface_name);
    iface->open(&arp_bpf);
    interfaces[iface->get_fd()] = iface;
    poller->add_fd(iface->get_fd());
}
