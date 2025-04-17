package api

// SystemNetwork defines a struct to hold the three types of supported network configuration.
type SystemNetwork struct {
	Version string `json:"version" yaml:"version"`

	Hostname string `json:"hostname" yaml:"hostname"`
	Domain   string `json:"domain"   yaml:"domain"`

	Interfaces []SystemNetworkInterface `json:"interfaces" yaml:"interfaces"`
	Bonds      []SystemNetworkBond      `json:"bonds"      yaml:"bonds"`
	Vlans      []SystemNetworkVlan      `json:"vlans"      yaml:"vlans"`
}

// SystemNetworkInterface contains information about a network interface.
type SystemNetworkInterface struct {
	Name      string               `json:"name"             yaml:"name"`
	VLAN      int                  `json:"vlan"             yaml:"vlan"`
	Addresses []string             `json:"addresses"        yaml:"addresses"`
	Routes    []SystemNetworkRoute `json:"routes,omitempty" yaml:"routes,omitempty"`
	Hwaddr    string               `json:"hwaddr"           yaml:"hwaddr"`
	Roles     []string             `json:"roles"            yaml:"roles"`
	LLDP      bool                 `json:"lldp"             yaml:"lldp"`
}

// SystemNetworkBond contains information about a network bond.
type SystemNetworkBond struct {
	Name      string               `json:"name"             yaml:"name"`
	Mode      string               `json:"mode"             yaml:"mode"`
	MTU       int                  `json:"mtu"              yaml:"mtu"`
	VLAN      int                  `json:"vlan"             yaml:"vlan"`
	Addresses []string             `json:"addresses"        yaml:"addresses"`
	Routes    []SystemNetworkRoute `json:"routes,omitempty" yaml:"routes,omitempty"`
	Members   []string             `json:"members"          yaml:"members"`
	Roles     []string             `json:"roles"            yaml:"roles"`
	LLDP      bool                 `json:"lldp"             yaml:"lldp"`
}

// SystemNetworkVlan contains information about a network vlan.
type SystemNetworkVlan struct {
	Name   string   `json:"name"   yaml:"name"`
	Parent string   `json:"parent" yaml:"parent"`
	ID     int      `json:"id"     yaml:"id"`
	MTU    int      `json:"mtu"    yaml:"mtu"`
	Roles  []string `json:"roles"  yaml:"roles"`
}

// SystemNetworkRoute defines a route.
type SystemNetworkRoute struct {
	To  string `json:"to"  yaml:"to"`
	Via string `json:"via" yaml:"via"`
}
