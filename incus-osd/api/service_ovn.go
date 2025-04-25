package api

// ServiceOVN represents the state and configuration of the OVN service.
type ServiceOVN struct {
	State struct{} `json:"state" yaml:"state"`

	Config struct {
		Enabled              bool   `json:"enabled"                yaml:"enabled"`
		ICChassis            bool   `json:"ic_chassis"             yaml:"ic_chassis"`
		Database             string `json:"database"               yaml:"database"`
		TLSClientCertificate string `json:"tls_client_certificate" yaml:"tls_client_certificate"`
		TLSClientKey         string `json:"tls_client_key"         yaml:"tls_client_key"`
		TLSCACertificate     string `json:"tls_ca_certificate"     yaml:"tls_ca_certificate"`
		TunnelAddress        string `json:"tunnel_address"         yaml:"tunnel_address"`
		TunnelProtocol       string `json:"tunnel_protocol"        yaml:"tunnel_protocol"`
	} `json:"config" yaml:"config"`
}
