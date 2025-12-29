package api

// ServiceOVNConfig represents additional configuration for the OVN service.
type ServiceOVNConfig struct {
	Enabled              bool   `json:"enabled"                          yaml:"enabled"`
	ICChassis            bool   `json:"ic_chassis,omitempty"             yaml:"ic_chassis,omitempty"`
	Database             string `json:"database"                         yaml:"database"`
	TLSClientCertificate string `json:"tls_client_certificate,omitempty" yaml:"tls_client_certificate,omitempty"`
	TLSClientKey         string `json:"tls_client_key,omitempty"         yaml:"tls_client_key,omitempty"`
	TLSCACertificate     string `json:"tls_ca_certificate,omitempty"     yaml:"tls_ca_certificate,omitempty"`
	TunnelAddress        string `json:"tunnel_address"                   yaml:"tunnel_address"`
	TunnelProtocol       string `json:"tunnel_protocol"                  yaml:"tunnel_protocol"`
}

// ServiceOVNState represents state for the OVN service.
type ServiceOVNState struct{}

// ServiceOVN represents the state and configuration of the OVN service.
type ServiceOVN struct {
	State ServiceOVNState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceOVNConfig `json:"config" yaml:"config"`
}
