package api

// SystemNetwork defines a struct to hold the three types of supported network configuration.
type SystemNetwork struct {
	Config *SystemNetworkConfig `json:"config" yaml:"config"`

	State SystemNetworkState `json:"state" yaml:"state"`
}

// SystemNetworkConfig represents the user modifiable network configuration.
type SystemNetworkConfig struct {
	DNS   *SystemNetworkDNS   `json:"dns"   yaml:"dns"`
	NTP   *SystemNetworkNTP   `json:"ntp"   yaml:"ntp"`
	Proxy *SystemNetworkProxy `json:"proxy" yaml:"proxy"`

	Interfaces []SystemNetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
	Bonds      []SystemNetworkBond      `json:"bonds,omitempty"      yaml:"bonds,omitempty"`
	VLANs      []SystemNetworkVLAN      `json:"vlans,omitempty"      yaml:"vlans,omitempty"`
}

// SystemNetworkInterface contains information about a network interface.
type SystemNetworkInterface struct {
	Name              string               `json:"name"                          yaml:"name"`
	MTU               int                  `json:"mtu"                           yaml:"mtu"`
	VLAN              int                  `json:"vlan"                          yaml:"vlan"`
	VLANTags          []int                `json:"vlan_tags,omitempty"           yaml:"vlan_tags,omitempty"`
	Addresses         []string             `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	RequiredForOnline string               `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Routes            []SystemNetworkRoute `json:"routes,omitempty"              yaml:"routes,omitempty"`
	Hwaddr            string               `json:"hwaddr"                        yaml:"hwaddr"`
	Roles             []string             `json:"roles,omitempty"               yaml:"roles,omitempty"`
	LLDP              bool                 `json:"lldp"                          yaml:"lldp"`
}

// SystemNetworkBond contains information about a network bond.
type SystemNetworkBond struct {
	Name              string               `json:"name"                          yaml:"name"`
	Mode              string               `json:"mode"                          yaml:"mode"`
	MTU               int                  `json:"mtu"                           yaml:"mtu"`
	VLAN              int                  `json:"vlan"                          yaml:"vlan"`
	VLANTags          []int                `json:"vlan_tags,omitempty"           yaml:"vlan_tags,omitempty"`
	Addresses         []string             `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	RequiredForOnline string               `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Routes            []SystemNetworkRoute `json:"routes,omitempty"              yaml:"routes,omitempty"`
	Hwaddr            string               `json:"hwaddr"                        yaml:"hwaddr"`
	Members           []string             `json:"members,omitempty"             yaml:"members,omitempty"`
	Roles             []string             `json:"roles,omitempty"               yaml:"roles,omitempty"`
	LLDP              bool                 `json:"lldp"                          yaml:"lldp"`
}

// SystemNetworkVLAN contains information about a network vlan.
type SystemNetworkVLAN struct {
	Name              string               `json:"name"                          yaml:"name"`
	Parent            string               `json:"parent"                        yaml:"parent"`
	ID                int                  `json:"id"                            yaml:"id"`
	MTU               int                  `json:"mtu"                           yaml:"mtu"`
	Addresses         []string             `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	RequiredForOnline string               `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Routes            []SystemNetworkRoute `json:"routes,omitempty"              yaml:"routes,omitempty"`
	Roles             []string             `json:"roles,omitempty"               yaml:"roles,omitempty"`
}

// SystemNetworkRoute defines a route.
type SystemNetworkRoute struct {
	To  string `json:"to"  yaml:"to"`
	Via string `json:"via" yaml:"via"`
}

// SystemNetworkDNS defines DNS configuration options.
type SystemNetworkDNS struct {
	Hostname      string   `json:"hostname"                 yaml:"hostname"`
	Domain        string   `json:"domain"                   yaml:"domain"`
	SearchDomains []string `json:"search_domains,omitempty" yaml:"search_domains,omitempty"`
	Nameservers   []string `json:"nameservers,omitempty"    yaml:"nameservers,omitempty"`
}

// SystemNetworkNTP defines static timeservers to use.
type SystemNetworkNTP struct {
	Timeservers []string `json:"timeservers,omitempty" yaml:"timeservers,omitempty"`
}

// SystemNetworkProxy defines proxy configuration.
type SystemNetworkProxy struct {
	HTTPProxy  string `json:"http_proxy"  yaml:"http_proxy"`
	HTTPSProxy string `json:"https_proxy" yaml:"https_proxy"`
	NoProxy    string `json:"no_proxy"    yaml:"no_proxy"`
}

// SystemNetworkState holds information about the current network state.
type SystemNetworkState struct {
	Interfaces map[string]SystemNetworkInterfaceState `json:"interfaces" yaml:"interfaces"`
}

// SystemNetworkInterfaceState holds state information about a specific network interface.
type SystemNetworkInterfaceState struct {
	Type      string                                 `json:"type,omitempty"      yaml:"type,omitempty"`
	Addresses []string                               `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	Routes    []SystemNetworkRoute                   `json:"routes,omitempty"    yaml:"routes,omitempty"`
	MTU       int                                    `json:"mtu"                 yaml:"mtu"`
	Speed     string                                 `json:"speed,omitempty"     yaml:"speed,omitempty"`
	State     string                                 `json:"state"               yaml:"state"`
	Stats     SystemNetworkInterfaceStats            `json:"stats"               yaml:"stats"`
	LLDP      []SystemNetworkLLDPState               `json:"lldp,omitempty"      yaml:"lldp,omitempty"`
	LACP      *SystemNetworkLACPState                `json:"lacp,omitempty"      yaml:"lacp,omitempty"`
	Members   map[string]SystemNetworkInterfaceState `json:"members,omitempty"   yaml:"members,omitempty"`
}

// SystemNetworkInterfaceStats holds RX/TX stats for an interface.
type SystemNetworkInterfaceStats struct {
	RXBytes  int `json:"rx_bytes"  yaml:"rx_bytes"`
	TXBytes  int `json:"tx_bytes"  yaml:"tx_bytes"`
	RXErrors int `json:"rx_errors" yaml:"rx_errors"`
	TXErrors int `json:"tx_errors" yaml:"tx_errors"`
}

// SystemNetworkLLDPState holds information about the LLDP state.
type SystemNetworkLLDPState struct {
	Name      string `json:"name"           yaml:"name"`
	ChassisID string `json:"chassis_id"     yaml:"chassis_id"`
	PortID    string `json:"port_id"        yaml:"port_id"`
	Port      string `json:"port,omitempty" yaml:"port,omitempty"`
}

// SystemNetworkLACPState holds information about a bond's LACP state.
type SystemNetworkLACPState struct {
	LocalMAC  string `json:"local_mac"  yaml:"local_mac"`
	RemoteMAC string `json:"remote_mac" yaml:"remote_mac"`
}
