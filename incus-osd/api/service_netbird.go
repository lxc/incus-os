package api

// ServiceNetbird represents the state and configuration of the Netbird service.
type ServiceNetbird struct {
	Config ServiceNetbirdConfig `json:"config" yaml:"config"`
	State  struct{}             `json:"state"  yaml:"state"`
}

// ServiceNetbirdConfig represents additional configuration for the Netbird service.
type ServiceNetbirdConfig struct {
	Enabled             bool     `json:"enabled"               yaml:"enabled"`
	SetupKey            string   `json:"setup_key"             yaml:"setup_key"`
	ManagementURL       string   `json:"management_url"        yaml:"management_url"`
	AdminURL            string   `json:"admin_url"             yaml:"admin_url"`
	Anonymize           bool     `json:"anonymize"             yaml:"anonymize"`
	BlockInbound        bool     `json:"block_inbound"         yaml:"block_inbound"`
	BlockLanAccess      bool     `json:"block_lan_access"      yaml:"block_lan_access"`
	DisableClientRoutes bool     `json:"disable_client_routes" yaml:"disable_client_routes"`
	DisableServerRoutes bool     `json:"disable_server_routes" yaml:"disable_server_routes"`
	DisableDNS          bool     `json:"disable_dns"           yaml:"disable_dns"`
	DisableFirewall     bool     `json:"disable_firewall"      yaml:"disable_firewall"`
	DNSResolverAddress  string   `json:"dns_resolver_address"  yaml:"dns_resolver_address"`
	ExternalIPMap       []string `json:"external_ip_map"       yaml:"external_ip_map"`
	ExtraDNSLabels      []string `json:"extra_dns_labels"      yaml:"extra_dns_labels"`
}
