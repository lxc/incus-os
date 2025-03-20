package seed

import (
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

// NetworkConfig defines a struct to hold the three types of supported network configuration.
type NetworkConfig struct {
	Interfaces []NetworkInterface `yaml:"interfaces"`
	Bonds      []NetworkBond      `yaml:"bonds"`
	Vlans      []NetworkVlan      `yaml:"vlans"`
}

// NetworkInterface contains information about a network interface.
type NetworkInterface struct {
	Name      string           `yaml:"name"`
	VLAN      int              `yaml:"vlan"`
	Addresses []CustomAddress  `yaml:"addresses"`
	Routes    []Route          `yaml:"routes,omitempty"`
	Hwaddr    net.HardwareAddr `yaml:"-"`
	Roles     []string         `yaml:"roles"`
}

// NetworkBond contains information about a network bond.
type NetworkBond struct {
	Name      string             `yaml:"name"`
	Mode      string             `yaml:"mode"`
	MTU       int                `yaml:"mtu"`
	VLAN      int                `yaml:"vlan"`
	Addresses []CustomAddress    `yaml:"addresses"`
	Routes    []Route            `yaml:"routes,omitempty"`
	Members   []net.HardwareAddr `yaml:"-"`
	Roles     []string           `yaml:"roles"`
}

// NetworkVlan contains information about a network vlan.
type NetworkVlan struct {
	Name   string   `yaml:"name"`
	Parent string   `yaml:"parent"`
	ID     int      `yaml:"id"`
	MTU    int      `yaml:"mtu"`
	Roles  []string `yaml:"roles"`
}

type addressType int

// Define constants for the different types of supported network address types.
const (
	AddressTypeRaw addressType = iota
	AddressTypeDHCP4
	AddressTypeDHCP6
	AddressTypeSLAAC
)

// CustomAddress defines a wrapper around net.IPNet so we can also handle DHCP/SLAAC addresses.
type CustomAddress struct {
	Type    addressType
	Address net.IPNet
}

// String implements the stringer interface.
func (c *CustomAddress) String() string {
	switch c.Type { //nolint:exhaustive
	case AddressTypeDHCP4:
		return "dhcp4"
	case AddressTypeDHCP6:
		return "dhcp6"
	case AddressTypeSLAAC:
		return "slaac"
	default:
		return c.Address.String()
	}
}

// Route defines a route.
type Route struct {
	To  net.IPNet     `yaml:"-"`
	Via CustomAddress `yaml:"via"`
}

// UnmarshalYAML implements custom unmarshaling for CustomAddress.
func (c *CustomAddress) UnmarshalYAML(node *yaml.Node) error {
	var raw string

	err := node.Decode(&raw)
	if err != nil {
		return err
	}

	switch strings.ToLower(raw) {
	case "dhcp4":
		c.Type = AddressTypeDHCP4
	case "dhcp6":
		c.Type = AddressTypeDHCP6
	case "slaac":
		c.Type = AddressTypeSLAAC
	default:
		if !strings.Contains(raw, "/") {
			if strings.Contains(raw, ":") {
				raw += "/128"
			} else {
				raw += "/32"
			}
		}

		ipAddr, ipNet, err := net.ParseCIDR(raw)
		if err != nil {
			return err
		}
		ipNet.IP = ipAddr

		c.Type = AddressTypeRaw
		c.Address = *ipNet
	}

	return nil
}

// MarshalYAML implements custom marshaling for CustomAddress.
func (c CustomAddress) MarshalYAML() (any, error) {
	return c.String(), nil
}

// UnmarshalYAML implements custom unmarshaling for Route.
func (r *Route) UnmarshalYAML(node *yaml.Node) error {
	type original Route
	raw := struct {
		original `yaml:",inline"`
		To       string `yaml:"to"`
	}{}

	err := node.Decode(&raw)
	if err != nil {
		return err
	}

	r.Via = raw.Via

	ipAddr, ipNet, err := net.ParseCIDR(raw.To)
	if err != nil {
		return err
	}
	ipNet.IP = ipAddr
	r.To = *ipNet

	return nil
}

// MarshalYAML implements custom marshaling for Route.
func (r Route) MarshalYAML() (any, error) {
	type original Route
	raw := struct {
		original `yaml:",inline"`
		To       string `yaml:"to"`
	}{
		original: original(r),
	}

	raw.To = r.To.String()

	return raw, nil
}

// UnmarshalYAML implements custom unmarshaling for NetworkInterface.
func (i *NetworkInterface) UnmarshalYAML(node *yaml.Node) error {
	type original NetworkInterface
	raw := struct {
		original `yaml:",inline"`
		Hwaddr   string `yaml:"hwaddr"`
	}{}

	err := node.Decode(&raw)
	if err != nil {
		return err
	}

	i.Name = raw.Name
	i.VLAN = raw.VLAN
	i.Addresses = raw.Addresses
	i.Routes = raw.Routes
	i.Roles = raw.Roles

	i.Hwaddr, err = net.ParseMAC(raw.Hwaddr)
	if err != nil {
		return err
	}

	return nil
}

// MarshalYAML implements custom marshaling for NetworkInterface.
func (i NetworkInterface) MarshalYAML() (any, error) {
	type original NetworkInterface
	raw := struct {
		original `yaml:",inline"`
		Hwaddr   string `yaml:"hwaddr"`
	}{
		original: original(i),
	}

	raw.Hwaddr = i.Hwaddr.String()

	return raw, nil
}

// UnmarshalYAML implements custom unmarshaling for NetworkBond.
func (b *NetworkBond) UnmarshalYAML(node *yaml.Node) error {
	type original NetworkBond
	raw := struct {
		original `yaml:",inline"`
		Members  []string `yaml:"members"`
	}{}

	err := node.Decode(&raw)
	if err != nil {
		return err
	}

	b.Name = raw.Name
	b.Mode = raw.Mode
	b.MTU = raw.MTU
	b.VLAN = raw.VLAN
	b.Addresses = raw.Addresses
	b.Routes = raw.Routes
	b.Roles = raw.Roles

	for _, member := range raw.Members {
		hwaddr, err := net.ParseMAC(member)
		if err != nil {
			return err
		}
		b.Members = append(b.Members, hwaddr)
	}

	return nil
}

// MarshalYAML implements custom marshaling for NetworkBond.
func (b NetworkBond) MarshalYAML() (any, error) {
	type original NetworkBond
	raw := struct {
		original `yaml:",inline"`
		Members  []string `yaml:"members"`
	}{
		original: original(b),
	}

	for _, member := range b.Members {
		raw.Members = append(raw.Members, member.String())
	}

	return raw, nil
}
