package network

import (
	"net/netip"

	"github.com/lxc/incus-os/incus-osd/api"
)

type linkFileVariables struct {
	Hwaddr    string
	RandomMAC bool
	Name      string
	MTU       int
	Ethernet  *api.SystemNetworkEthernet
}

type netdevFileVariables struct {
	Type           string
	Name           string
	Hwaddr         string
	StrippedHwaddr string
	Bridge         *api.SystemNetworkBridge
	BondMode       string
	BondOptions    *api.SystemNetworkBondOptions
	VLANID         int
	WGPrivateKey   string
	WGPort         int
	WGPeers        []api.SystemNetworkWireguardPeer
}

type networkFileVariables struct {
	Type              string
	Name              string
	RequiredForOnline string
	MTU               int
	VLANs             []api.SystemNetworkVLAN
	DNS               *api.SystemNetworkDNS
	TimeConfig        *api.SystemNetworkTime
	LLDP              string
	Bond              string
	Bridge            string
	Addresses         []string
	Routes            []api.SystemNetworkRoute
	VLANTags          []int
}

// HasIPv4 reports whether the network configures any IPv4 addressing.
func (v networkFileVariables) HasIPv4() bool {
	for _, address := range v.Addresses {
		if address == "dhcp4" {
			return true
		}

		prefix, err := netip.ParsePrefix(address)
		if err == nil && prefix.Addr().Is4() {
			return true
		}
	}

	return false
}
