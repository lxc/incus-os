package api

// ServiceLinstorConfig represents the Linstor service configuration.
type ServiceLinstorConfig struct {
	Enabled                bool     `json:"enabled"                  yaml:"enabled"`
	ListenAddress          string   `json:"listen_address"           yaml:"listen_address"`
	TLSServerCertificate   string   `json:"tls_server_certificate"   yaml:"tls_server_certificate"`
	TLSServerKey           string   `json:"tls_server_key"           yaml:"tls_server_key"`
	TLSTrustedCertificates []string `json:"tls_trusted_certificates" yaml:"tls_trusted_certificates"`
}

// ServiceLinstorState represents state for the Linstor service.
type ServiceLinstorState struct{}

// ServiceLinstor represents the state and configuration of the Linstor service.
type ServiceLinstor struct {
	State ServiceLinstorState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceLinstorConfig `json:"config" yaml:"config"`
}
