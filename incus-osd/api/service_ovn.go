package api

// ServiceOVNConfig represents additional configuration for the OVN service.
type ServiceOVNConfig struct {
	Enabled              bool   `json:"enabled"                yaml:"enabled"`
	ICChassis            bool   `json:"ic_chassis"             yaml:"ic_chassis"`
	Database             string `json:"database"               yaml:"database"`
	TLSClientCertificate string `json:"tls_client_certificate" yaml:"tls_client_certificate"`
	TLSClientKey         string `json:"tls_client_key"         yaml:"tls_client_key"`
	TLSCACertificate     string `json:"tls_ca_certificate"     yaml:"tls_ca_certificate"`
	TunnelAddress        string `json:"tunnel_address"         yaml:"tunnel_address"`
	TunnelProtocol       string `json:"tunnel_protocol"        yaml:"tunnel_protocol"`
}

// ServiceOVNState represents state for the OVN service.
type ServiceOVNState struct{}

// ServiceOVN represents the state and configuration of the OVN service.
type ServiceOVN struct {
	State ServiceOVNState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceOVNConfig `json:"config" yaml:"config"`
}
