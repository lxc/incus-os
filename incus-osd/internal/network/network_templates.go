package network

import (
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
	BondMode       string
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
