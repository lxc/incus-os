package api

import (
	"net"
	"slices"
)

const (
	// SystemNetworkInterfaceRoleManagement represents the "management" role.
	SystemNetworkInterfaceRoleManagement = "management"

	// SystemNetworkInterfaceRoleCluster represents the "cluster" role.
	SystemNetworkInterfaceRoleCluster = "cluster"

	// SystemNetworkInterfaceRoleInstances represents the "instances" role.
	SystemNetworkInterfaceRoleInstances = "instances"

	// SystemNetworkInterfaceRoleStorage represents the "storage" role.
	SystemNetworkInterfaceRoleStorage = "storage"
)

// SystemNetwork defines a struct to hold the three types of supported network configuration.
type SystemNetwork struct {
	Config *SystemNetworkConfig `json:"config" yaml:"config"`

	State SystemNetworkState `incusos:"-" json:"state" yaml:"state"`
}

// SystemNetworkConfig represents the user modifiable network configuration.
type SystemNetworkConfig struct {
	DNS   *SystemNetworkDNS   `json:"dns,omitempty"   yaml:"dns,omitempty"`
	Time  *SystemNetworkTime  `json:"time,omitempty"  yaml:"time,omitempty"`
	Proxy *SystemNetworkProxy `json:"proxy,omitempty" yaml:"proxy,omitempty"`

	Interfaces []SystemNetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
	Bonds      []SystemNetworkBond      `json:"bonds,omitempty"      yaml:"bonds,omitempty"`
	VLANs      []SystemNetworkVLAN      `json:"vlans,omitempty"      yaml:"vlans,omitempty"`
	Wireguard  []SystemNetworkWireguard `json:"wireguard,omitempty"  yaml:"wireguard,omitempty"`
}

// SystemNetworkInterface contains information about a network interface.
type SystemNetworkInterface struct {
	Addresses         []string                    `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	Ethernet          *SystemNetworkEthernet      `json:"ethernet,omitempty"            yaml:"ethernet,omitempty"`
	FirewallRules     []SystemNetworkFirewallRule `json:"firewall_rules,omitempty"      yaml:"firewall_rules,omitempty"`
	Hwaddr            string                      `json:"hwaddr"                        yaml:"hwaddr"`
	LLDP              bool                        `json:"lldp,omitempty"                yaml:"lldp,omitempty"`
	MTU               int                         `json:"mtu,omitempty"                 yaml:"mtu,omitempty"`
	Name              string                      `json:"name"                          yaml:"name"`
	RequiredForOnline string                      `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Roles             []string                    `json:"roles,omitempty"               yaml:"roles,omitempty"`
	Routes            []SystemNetworkRoute        `json:"routes,omitempty"              yaml:"routes,omitempty"`
	StrictHwaddr      bool                        `json:"strict_hwaddr,omitempty"       yaml:"strict_hwaddr,omitempty"`
	VLANTags          []int                       `json:"vlan_tags,omitempty"           yaml:"vlan_tags,omitempty"`
}

// SystemNetworkBond contains information about a network bond.
type SystemNetworkBond struct {
	Addresses         []string                    `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	Ethernet          *SystemNetworkEthernet      `json:"ethernet,omitempty"            yaml:"ethernet,omitempty"`
	FirewallRules     []SystemNetworkFirewallRule `json:"firewall_rules,omitempty"      yaml:"firewall_rules,omitempty"`
	Hwaddr            string                      `json:"hwaddr,omitempty"              yaml:"hwaddr,omitempty"`
	LLDP              bool                        `json:"lldp,omitempty"                yaml:"lldp,omitempty"`
	Members           []string                    `json:"members,omitempty"             yaml:"members,omitempty"`
	Mode              string                      `json:"mode"                          yaml:"mode"`
	MTU               int                         `json:"mtu,omitempty"                 yaml:"mtu,omitempty"`
	Name              string                      `json:"name"                          yaml:"name"`
	RequiredForOnline string                      `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Roles             []string                    `json:"roles,omitempty"               yaml:"roles,omitempty"`
	Routes            []SystemNetworkRoute        `json:"routes,omitempty"              yaml:"routes,omitempty"`
	VLANTags          []int                       `json:"vlan_tags,omitempty"           yaml:"vlan_tags,omitempty"`
}

// SystemNetworkVLAN contains information about a network vlan.
type SystemNetworkVLAN struct {
	Addresses         []string                    `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	FirewallRules     []SystemNetworkFirewallRule `json:"firewall_rules,omitempty"      yaml:"firewall_rules,omitempty"`
	ID                int                         `json:"id"                            yaml:"id"`
	MTU               int                         `json:"mtu,omitempty"                 yaml:"mtu,omitempty"`
	Name              string                      `json:"name"                          yaml:"name"`
	Parent            string                      `json:"parent"                        yaml:"parent"`
	RequiredForOnline string                      `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Roles             []string                    `json:"roles,omitempty"               yaml:"roles,omitempty"`
	Routes            []SystemNetworkRoute        `json:"routes,omitempty"              yaml:"routes,omitempty"`
}

// SystemNetworkEthernet contains Ethernet-specific configuration details (offloading and other features).
type SystemNetworkEthernet struct {
	DisableEnergyEfficient bool     `json:"disable_energy_efficient,omitempty" yaml:"disable_energy_efficient,omitempty"`
	DisableGRO             bool     `json:"disable_gro,omitempty"              yaml:"disable_gro,omitempty"`
	DisableGSO             bool     `json:"disable_gso,omitempty"              yaml:"disable_gso,omitempty"`
	DisableIPv4TSO         bool     `json:"disable_ipv4_tso,omitempty"         yaml:"disable_ipv4_tso,omitempty"`
	DisableIPv6TSO         bool     `json:"disable_ipv6_tso,omitempty"         yaml:"disable_ipv6_tso,omitempty"`
	WakeOnLAN              bool     `json:"wakeonlan,omitempty"                yaml:"wakeonlan,omitempty"`
	WakeOnLANModes         []string `json:"wakeonlan_modes,omitempty"          yaml:"wakeonlan_modes,omitempty"`
	WakeOnLANPassword      string   `json:"wakeonlan_password,omitempty"       yaml:"wakeonlan_password,omitempty"`
}

// SystemNetworkFirewallRule defines a firewall rule.
type SystemNetworkFirewallRule struct {
	Action   string `json:"action"             yaml:"action"`
	Source   string `json:"source,omitempty"   yaml:"source,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Port     int    `json:"port,omitempty"     yaml:"port,omitempty"`
}

// SystemNetworkWireguard contains information about a wireguard interface.
type SystemNetworkWireguard struct {
	Addresses         []string                     `json:"addresses,omitempty"           yaml:"addresses,omitempty"`
	FirewallRules     []SystemNetworkFirewallRule  `json:"firewall_rules,omitempty"      yaml:"firewall_rules,omitempty"`
	MTU               int                          `json:"mtu,omitempty"                 yaml:"mtu,omitempty"`
	Name              string                       `json:"name"                          yaml:"name"`
	Peers             []SystemNetworkWireguardPeer `json:"peers,omitempty"               yaml:"peers,omitempty"`
	Port              int                          `json:"port,omitempty"                yaml:"port,omitempty"`
	PrivateKey        string                       `json:"private_key,omitempty"         yaml:"private_key,omitempty"` //nolint:gosec
	RequiredForOnline string                       `json:"required_for_online,omitempty" yaml:"required_for_online,omitempty"`
	Roles             []string                     `json:"roles,omitempty"               yaml:"roles,omitempty"`
	Routes            []SystemNetworkRoute         `json:"routes,omitempty"              yaml:"routes,omitempty"`
}

// SystemNetworkWireguardPeer defines wireguard peer.
type SystemNetworkWireguardPeer struct {
	AllowedIPs          []string `json:"allowed_ips"                    yaml:"allowed_ips"`
	Endpoint            string   `json:"endpoint,omitempty"             yaml:"endpoint,omitempty"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty" yaml:"persistent_keepalive,omitempty"`
	PresharedKey        string   `json:"preshared_key,omitempty"        yaml:"preshared_key,omitempty"`
	PublicKey           string   `json:"public_key"                     yaml:"public_key"`
}

// SystemNetworkRoute defines a route.
type SystemNetworkRoute struct {
	To  string `json:"to"  yaml:"to"`
	Via string `json:"via" yaml:"via"`
}

// SystemNetworkDNS defines DNS configuration options.
type SystemNetworkDNS struct {
	Domain        string   `json:"domain"                   yaml:"domain"`
	Hostname      string   `json:"hostname"                 yaml:"hostname"`
	Nameservers   []string `json:"nameservers,omitempty"    yaml:"nameservers,omitempty"`
	SearchDomains []string `json:"search_domains,omitempty" yaml:"search_domains,omitempty"`
}

// SystemNetworkTime defines various time related configuration options (NTP servers, timezone, etc).
type SystemNetworkTime struct {
	NTPServers []string `json:"ntp_servers,omitempty" yaml:"ntp_servers,omitempty"`
	Timezone   string   `json:"timezone,omitempty"    yaml:"timezone,omitempty"`
}

// SystemNetworkProxy defines proxy configuration.
type SystemNetworkProxy struct {
	Rules   []SystemNetworkProxyRule            `json:"rules,omitempty"   yaml:"rules,omitempty"`
	Servers map[string]SystemNetworkProxyServer `json:"servers,omitempty" yaml:"servers,omitempty"`
}

// SystemNetworkProxyServer defines a proxy server configuration.
type SystemNetworkProxyServer struct {
	Auth     string `json:"auth"               yaml:"auth"`
	Host     string `json:"host"               yaml:"host"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"` //nolint:gosec
	Realm    string `json:"realm,omitempty"    yaml:"realm,omitempty"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	UseTLS   bool   `json:"use_tls"            yaml:"use_tls"`
}

// SystemNetworkProxyRule defines a proxy rule.
type SystemNetworkProxyRule struct {
	Destination string `json:"destination" yaml:"destination"`
	Target      string `json:"target"      yaml:"target"`
}

// SystemNetworkState holds information about the current network state.
type SystemNetworkState struct {
	Interfaces map[string]SystemNetworkInterfaceState `json:"interfaces" yaml:"interfaces"`
}

// GetInterfaceNamesByRole returns a slice of interface names that have the given role applied to them.
func (n *SystemNetworkState) GetInterfaceNamesByRole(role string) []string {
	names := []string{}

	for name, iState := range n.Interfaces {
		if slices.Contains(iState.Roles, role) {
			names = append(names, name)
		}
	}

	return names
}

// GetInterfaceAddressByRole returns the preferred IP address from an interface with the provided role.
// A nil value is returned if none could be found.
func (n *SystemNetworkState) GetInterfaceAddressByRole(role string) net.IP {
	if len(n.Interfaces) == 0 {
		return nil
	}

	var (
		ipv4Address net.IP
		ipv6Address net.IP
	)

	for _, iface := range n.Interfaces {
		// Skip if missing role.
		if !slices.Contains(iface.Roles, role) {
			continue
		}

		for _, address := range iface.Addresses {
			addrIP := net.ParseIP(address)
			if addrIP == nil {
				continue
			}

			if addrIP.To4() == nil {
				if ipv6Address == nil {
					ipv6Address = addrIP
				}
			} else {
				if ipv4Address == nil {
					ipv4Address = addrIP
				}
			}
		}

		// Break early if we have an IPv6 address as we'll prefer that anyway.
		if ipv6Address != nil {
			break
		}
	}

	if ipv6Address != nil {
		return ipv6Address
	}

	if ipv4Address != nil {
		return ipv4Address
	}

	return nil
}

// SystemNetworkInterfaceState holds state information about a specific network interface.
type SystemNetworkInterfaceState struct {
	Addresses []string                               `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	Hwaddr    string                                 `json:"hwaddr,omitempty"    yaml:"hwaddr,omitempty"`
	LACP      *SystemNetworkLACPState                `json:"lacp,omitempty"      yaml:"lacp,omitempty"`
	LLDP      []SystemNetworkLLDPState               `json:"lldp,omitempty"      yaml:"lldp,omitempty"`
	Members   map[string]SystemNetworkInterfaceState `json:"members,omitempty"   yaml:"members,omitempty"`
	MTU       int                                    `json:"mtu,omitempty"       yaml:"mtu,omitempty"`
	Roles     []string                               `json:"roles,omitempty"     yaml:"roles,omitempty"`
	Routes    []SystemNetworkRoute                   `json:"routes,omitempty"    yaml:"routes,omitempty"`
	Speed     string                                 `json:"speed,omitempty"     yaml:"speed,omitempty"`
	State     string                                 `json:"state"               yaml:"state"`
	Stats     SystemNetworkInterfaceStats            `json:"stats"               yaml:"stats"`
	Type      string                                 `json:"type,omitempty"      yaml:"type,omitempty"`
	Wireguard *SystemNetworkWireguardState           `json:"wireguard,omitempty" yaml:"wireguard,omitempty"`
}

// SystemNetworkInterfaceStats holds RX/TX stats for an interface.
type SystemNetworkInterfaceStats struct {
	RXBytes  int `json:"rx_bytes"  yaml:"rx_bytes"`
	RXErrors int `json:"rx_errors" yaml:"rx_errors"`
	TXBytes  int `json:"tx_bytes"  yaml:"tx_bytes"`
	TXErrors int `json:"tx_errors" yaml:"tx_errors"`
}

// SystemNetworkLLDPState holds information about the LLDP state.
type SystemNetworkLLDPState struct {
	ChassisID string `json:"chassis_id"     yaml:"chassis_id"`
	Name      string `json:"name"           yaml:"name"`
	PortID    string `json:"port_id"        yaml:"port_id"`
	Port      string `json:"port,omitempty" yaml:"port,omitempty"`
}

// SystemNetworkLACPState holds information about a bond's LACP state.
type SystemNetworkLACPState struct {
	LocalMAC  string `json:"local_mac"  yaml:"local_mac"`
	RemoteMAC string `json:"remote_mac" yaml:"remote_mac"`
}

// SystemNetworkWireguardState holds state information about a specific wireguard interface.
type SystemNetworkWireguardState struct {
	ListeningPort int                               `json:"listening_port,omitempty" yaml:"listening_port,omitempty"`
	Peers         []SystemNetworkWireguardPeerState `json:"peers,omitempty"          yaml:"peers,omitempty"`
	PublicKey     string                            `json:"public_key"               yaml:"public_key"`
}

// SystemNetworkWireguardPeerState holds state information about a specific wireguard peer.
type SystemNetworkWireguardPeerState struct {
	AllowedIPs          []string                    `json:"allowed_ips"                    yaml:"allowed_ips"`
	EndPoint            string                      `json:"endpoint"                       yaml:"endpoint"`
	LatestHandshake     string                      `json:"latest_handshake,omitempty"     yaml:"latest_handshake,omitempty"`
	PersistentKeepalive string                      `json:"persistent_keepalive,omitempty" yaml:"persistent_keepalive,omitempty"`
	PublicKey           string                      `json:"public_key"                     yaml:"public_key"`
	Stats               SystemNetworkInterfaceStats `json:"stats"                          yaml:"stats"`
}
