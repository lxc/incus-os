package systemd

import (
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

type AddressType int

const (
	AddressTypeRaw AddressType = iota
	AddressTypeDHCP4
	AddressTypeDHCP6
	AddressTypeSLAAC
)

type CustomAddress struct {
	Type    AddressType
	Address net.IPNet
}

// Implement the stringer interface.
func (c CustomAddress) String() string {
	switch c.Type {
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

type Route struct {
	To  net.IPNet     `yaml:"-"`
	Via CustomAddress `yaml:"via"`
}

type NetworkConfig struct {
	Interfaces []NetworkInterfaces `yaml:"interfaces"`
	Bonds      []NetworkBonds      `yaml:"bonds"`
	Vlans      []NetworkVlans      `yaml:"vlans"`
}

type NetworkInterfaces struct {
	Name      string           `yaml:"name"`
	VLAN      int              `yaml:"vlan"`
	Addresses []CustomAddress  `yaml:"addresses"`
	Routes    []Route          `yaml:"routes,omitempty"`
	Hwaddr    net.HardwareAddr `yaml:"-"`
	Roles     []string         `yaml:"roles"`
}

type NetworkBonds struct {
	Name      string             `yaml:"name"`
	Mode      string             `yaml:"mode"`
	MTU       int                `yaml:"mtu"`
	VLAN      int                `yaml:"vlan"`
	Addresses []CustomAddress    `yaml:"addresses"`
	Routes    []Route            `yaml:"routes,omitempty"`
	Members   []net.HardwareAddr `yaml:"-"`
	Roles     []string           `yaml:"roles"`
}

type NetworkVlans struct {
	Name   string   `yaml:"name"`
	Parent string   `yaml:"parent"`
	ID     int      `yaml:"id"`
	MTU    int      `yaml:"mtu"`
	Roles  []string `yaml:"roles"`
}

func (c *CustomAddress) UnmarshalYAML(node *yaml.Node) error {
	var raw string

	err := node.Decode(&raw)
	if err != nil {
		return err
	}

	if strings.ToLower(raw) == "dhcp4" {
		c.Type = AddressTypeDHCP4
	} else if strings.ToLower(raw) == "dhcp6" {
		c.Type = AddressTypeDHCP6
	} else if strings.ToLower(raw) == "slaac" {
		c.Type = AddressTypeSLAAC
	} else {
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

func (c CustomAddress) MarshalYAML() (any, error) {
	return c.String(), nil
}

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

func (i *NetworkInterfaces) UnmarshalYAML(node *yaml.Node) error {
	type original NetworkInterfaces
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

func (i NetworkInterfaces) MarshalYAML() (any, error) {
	type original NetworkInterfaces
	raw := struct {
		original `yaml:",inline"`
		Hwaddr   string `yaml:"hwaddr"`
	}{
		original: original(i),
	}

	raw.Hwaddr = i.Hwaddr.String()

	return raw, nil
}

func (b *NetworkBonds) UnmarshalYAML(node *yaml.Node) error {
	type original NetworkBonds
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

func (b NetworkBonds) MarshalYAML() (any, error) {
	type original NetworkBonds
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
