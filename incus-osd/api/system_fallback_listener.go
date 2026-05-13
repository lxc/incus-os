package api

// SystemFallbackListenerState holds information about the current fallback listener state.
type SystemFallbackListenerState struct {
	Active bool `incusos:"-" json:"active" yaml:"active"`
}

// SystemFallbackListenerConfig holds fallback listener configuration settings.
type SystemFallbackListenerConfig struct {
	ListenAddress             string   `json:"listen_address,omitempty"              yaml:"listen_address,omitempty"`              // If defined, listen on the specified IP:port address, otherwise attempt to listen on all interfaces on a random port.
	TrustedClientCertificates []string `json:"trusted_client_certificates,omitempty" yaml:"trusted_client_certificates,omitempty"` // A list of PEM-encoded trusted client certificates.
}

// SystemFallbackListener defines a struct to configure the fallback HTTPS listener that will
// activate if the primary application fails to start, ensuring that basic API connectivity
// to the system isn't lost.
type SystemFallbackListener struct {
	Config SystemFallbackListenerConfig `json:"config" yaml:"config"`

	State SystemFallbackListenerState `json:"state" yaml:"state"`
}
