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
	Enabled          bool     `json:"enabled"           yaml:"enabled"`
	LoginServer      string   `json:"login_server"      yaml:"login_server"`
	AuthKey          string   `json:"auth_key"          yaml:"auth_key"`
	AcceptRoutes     bool     `json:"accept_routes"     yaml:"accept_routes"`
	AdvertisedRoutes []string `json:"advertised_routes" yaml:"advertised_routes"`
	ServeEnabled     bool     `json:"serve_enabled"     yaml:"serve_enabled"`
	ServePort        int      `json:"serve_port"        yaml:"serve_port"`
	ServeService     string   `json:"serve_service"     yaml:"serve_service"`
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
	ID           string       `json:"id"`
	PublicKey    string       `json:"public_key"`
	HostName     string       `json:"host_name"`
	DNSName      string       `json:"dns_name"`
	OS           string       `json:"os"`
	TailscaleIPs []netip.Addr `json:"tailscale_ips"`
	RxBytes      int64        `json:"rx_bytes"`
	TxBytes      int64        `json:"tx_bytes"`
	Online       bool         `json:"online"`
	Expired      bool         `json:"expired"`
}

// ServiceTailscaleState represents the current state of the Tailscale service.
type ServiceTailscaleState struct {
	Version          string                           `json:"version"`
	BackendState     ServiceTailscaleBackendStateEnum `json:"backend_state"`
	TailnetName      string                           `json:"tailnet_name,omitempty"`
	TailnetDNSSuffix string                           `json:"tailnet_dns_suffix,omitempty"`
	Self             ServiceTailscaleStatePeer        `json:"self"`
	Peers            []ServiceTailscaleStatePeer      `json:"peer,omitempty"`
	HaveNodeKey      bool                             `json:"have_node_key"`
	Health           []string                         `json:"health,omitempty"`
}
