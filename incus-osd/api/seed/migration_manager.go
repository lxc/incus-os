package seed

import (
	"github.com/FuturFusion/migration-manager/shared/api"
)

// MigrationManager represents a Migration Manager seed file.
type MigrationManager struct {
	// A list of PEM-encoded trusted client certificates. The SHA256 fingerprint of
	// each certificate will be added to the list of any SHA256 fingerprints provided
	// in SystemSecurity.TrustedTLSClientCertFingerprints.
	TrustedClientCertificates []string `json:"trusted_client_certificates,omitempty" yaml:"trusted_client_certificates,omitempty"`

	SystemCertificate *api.SystemCertificatePost `json:"system_certificate,omitempty" yaml:"system_certificate,omitempty"`
	SystemNetwork     *api.SystemNetwork         `json:"system_network,omitempty"     yaml:"system_network,omitempty"`
	SystemSecurity    *api.SystemSecurity        `json:"system_security,omitempty"    yaml:"system_security,omitempty"`
}
