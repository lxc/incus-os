package seed

import (
	"github.com/FuturFusion/migration-manager/shared/api"
)

// MigrationManager represents a Migration Manager seed file.
type MigrationManager struct {
	Version string `json:"version" yaml:"version"`

	// A list of PEM-encoded trusted client certificates. The SHA256 fingerprint of
	// each certificate will be added to the list of any SHA256 fingerprints provided
	// in SystemSecurity.TrustedTLSClientCertFingerprints.
	TrustedClientCertificates []string `json:"trusted_client_certificates,omitempty" yaml:"trusted_client_certificates,omitempty"`

	ApplyDefaults bool                     `json:"apply_defaults" yaml:"apply_defaults"`
	Preseed       *MigrationManagerPreseed `json:"preseed"        yaml:"preseed"`
}

// MigrationManagerPreseed holds seed configuration for Migration Manager.
type MigrationManagerPreseed struct {
	SystemCertificate *api.SystemCertificatePost `json:"system_certificate,omitempty" yaml:"system_certificate,omitempty"`
	SystemNetwork     *api.SystemNetwork         `json:"system_network,omitempty"     yaml:"system_network,omitempty"`
	SystemSecurity    *api.SystemSecurity        `json:"system_security,omitempty"    yaml:"system_security,omitempty"`
}
