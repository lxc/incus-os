package seed

import (
	"github.com/FuturFusion/operations-center/shared/api"
)

// OperationsCenter represents an Operations Center seed file.
type OperationsCenter struct {
	Version string `json:"version" yaml:"version"`

	// A list of PEM-encoded trusted client certificates. The SHA256 fingerprint of
	// each certificate will be added to the list of any SHA256 fingerprints provided
	// in SystemSecurity.TrustedTLSClientCertFingerprints.
	TrustedClientCertificates []string `json:"trusted_client_certificates,omitempty" yaml:"trusted_client_certificates,omitempty"`

	ApplyDefaults bool                     `json:"apply_defaults" yaml:"apply_defaults"`
	Preseed       *OperationsCenterPreseed `json:"preseed"        yaml:"preseed"`
}

// OperationsCenterPreseed holds seed configuration for Operations Center.
type OperationsCenterPreseed struct {
	SystemCertificate *api.SystemCertificatePost `json:"system_certificate,omitempty" yaml:"system_certificate,omitempty"`
	SystemNetwork     *api.SystemNetworkPut      `json:"system_network,omitempty"     yaml:"system_network,omitempty"`
	SystemSecurity    *api.SystemSecurityPut     `json:"system_security,omitempty"    yaml:"system_security,omitempty"`
	SystemUpdates     *api.SystemUpdatesPut      `json:"system_updates,omitempty"     yaml:"system_updates,omitempty"`
}
