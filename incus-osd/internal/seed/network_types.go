package seed

// NetworkConfig defines a struct to hold the three types of supported network configuration.
type NetworkConfig struct {
	Version string `json:"version" yaml:"version"`

	Interfaces []NetworkInterface `json:"interfaces" yaml:"interfaces"`
	Bonds      []NetworkBond      `json:"bonds"      yaml:"bonds"`
	Vlans      []NetworkVlan      `json:"vlans"      yaml:"vlans"`
}

// NetworkInterface contains information about a network interface.
type NetworkInterface struct {
	Name      string   `json:"name"             yaml:"name"`
	VLAN      int      `json:"vlan"             yaml:"vlan"`
	Addresses []string `json:"addresses"        yaml:"addresses"`
	Routes    []Route  `json:"routes,omitempty" yaml:"routes,omitempty"`
	Hwaddr    string   `json:"hwaddr"           yaml:"hwaddr"`
	Roles     []string `json:"roles"            yaml:"roles"`
}

// NetworkBond contains information about a network bond.
type NetworkBond struct {
	Name      string   `json:"name"             yaml:"name"`
	Mode      string   `json:"mode"             yaml:"mode"`
	MTU       int      `json:"mtu"              yaml:"mtu"`
	VLAN      int      `json:"vlan"             yaml:"vlan"`
	Addresses []string `json:"addresses"        yaml:"addresses"`
	Routes    []Route  `json:"routes,omitempty" yaml:"routes,omitempty"`
	Members   []string `json:"members"          yaml:"members"`
	Roles     []string `json:"roles"            yaml:"roles"`
}

// NetworkVlan contains information about a network vlan.
type NetworkVlan struct {
	Name   string   `json:"name"   yaml:"name"`
	Parent string   `json:"parent" yaml:"parent"`
	ID     int      `json:"id"     yaml:"id"`
	MTU    int      `json:"mtu"    yaml:"mtu"`
	Roles  []string `json:"roles"  yaml:"roles"`
}

// Route defines a route.
type Route struct {
	To  string `json:"to"  yaml:"to"`
	Via string `json:"via" yaml:"via"`
}
