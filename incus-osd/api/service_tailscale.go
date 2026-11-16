package api

import (
	"net/netip"
)

// ServiceTailscale represents the state and configuration of the Tailscale service.
type ServiceTailscale struct {
	Config ServiceTailscaleConfig `json:"config" yaml:"config"`
	State  ServiceTailscaleState  `incusos:"-"   json:"state"  yaml:"state"`
}

// ServiceTailscaleConfig represents additional configuration for the Tailscale service.
type ServiceTailscaleConfig struct {
	Enabled                bool     `json:"enabled"                    yaml:"enabled"`
	LoginServer            string   `json:"login_server"               yaml:"login_server"`
	AuthKey                string   `json:"auth_key"                   yaml:"auth_key"`
	Port                   int      `json:"port"                       yaml:"port"`
	AcceptRoutes           bool     `json:"accept_routes"              yaml:"accept_routes"`
	AcceptDNS              bool     `json:"accept_dns"                 yaml:"accept_dns"`
	AdvertisedRoutes       []string `json:"advertised_routes"          yaml:"advertised_routes"`
	AdvertiseExitNode      bool     `json:"advertise_exit_node"        yaml:"advertise_exit_node"`
	ExitNode               string   `json:"exit_node"                  yaml:"exit_node"`
	ExitNodeAllowLanAccess bool     `json:"exit_node_allow_lan_access" yaml:"exit_node_allow_lan_access"`
	ServeEnabled           bool     `json:"serve_enabled"              yaml:"serve_enabled"`
	ServePort              int      `json:"serve_port"                 yaml:"serve_port"`
	ServeService           string   `json:"serve_service"              yaml:"serve_service"`
}

// ServiceTailscaleBackendStateEnum represents the possible states of the Tailscale backend.
type ServiceTailscaleBackendStateEnum string

// Possible values for ServiceTailscaleBackendStateEnum.
const (
	ServiceTailscaleBackendStateNoState          ServiceTailscaleBackendStateEnum = "NoState"
	ServiceTailscaleBackendStateNeedsLogin       ServiceTailscaleBackendStateEnum = "NeedsLogin"
	ServiceTailscaleBackendStateNeedsMachineAuth ServiceTailscaleBackendStateEnum = "NeedsMachineAuth"
	ServiceTailscaleBackendStateStopped          ServiceTailscaleBackendStateEnum = "Stopped"
	ServiceTailscaleBackendStateStarting         ServiceTailscaleBackendStateEnum = "Starting"
	ServiceTailscaleBackendStateRunning          ServiceTailscaleBackendStateEnum = "Running"
)

// ServiceTailscaleStatePeer represents the state of a single peer in the Tailscale network.
type ServiceTailscaleStatePeer struct {
	ID              string       `json:"id"                yaml:"id"`
	PublicKey       string       `json:"public_key"        yaml:"public_key"`
	HostName        string       `json:"host_name"         yaml:"host_name"`
	DNSName         string       `json:"dns_name"          yaml:"dns_name"`
	OS              string       `json:"os"                yaml:"os"`
	TailscaleIPs    []netip.Addr `json:"tailscale_ips"     yaml:"tailscale_ips"`
	RxBytes         int64        `json:"rx_bytes"          yaml:"rx_bytes"`
	TxBytes         int64        `json:"tx_bytes"          yaml:"tx_bytes"`
	UsedAsExitNode  bool         `json:"used_as_exit_node" yaml:"used_as_exit_node"`
	ExitNodeOffered bool         `json:"exit_node_offered" yaml:"exit_node_offered"`
	Online          bool         `json:"online"            yaml:"online"`
	Expired         bool         `json:"expired"           yaml:"expired"`
}

// ServiceTailscaleState represents the current state of the Tailscale service.
type ServiceTailscaleState struct {
	Version          string                           `json:"version"                      yaml:"version"`
	BackendState     ServiceTailscaleBackendStateEnum `json:"backend_state"                yaml:"backend_state"`
	TailnetName      string                           `json:"tailnet_name,omitempty"       yaml:"tailnet_name,omitempty"`
	TailnetDNSSuffix string                           `json:"tailnet_dns_suffix,omitempty" yaml:"tailnet_dns_suffix,omitempty"`
	Self             ServiceTailscaleStatePeer        `json:"self"                         yaml:"self"`
	Peers            []ServiceTailscaleStatePeer      `json:"peers,omitempty"              yaml:"peers,omitempty"`
	HaveNodeKey      bool                             `json:"have_node_key"                yaml:"have_node_key"`
	Health           []string                         `json:"health,omitempty"             yaml:"health,omitempty"`
}
