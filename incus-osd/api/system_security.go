package api

// SystemSecurity defines a struct to hold information about the system's security state.
type SystemSecurity struct {
	Config struct {
		EncryptionRecoveryKeys []string `json:"encryption_recovery_keys" yaml:"encryption_recovery_keys"`
	} `json:"config" yaml:"config"`

	State struct {
		EncryptionRecoveryKeysRetrieved bool                                  `json:"encryption_recovery_keys_retrieved" yaml:"encryption_recovery_keys_retrieved"`
		SecureBootEnabled               bool                                  `json:"secure_boot_enabled"                yaml:"secure_boot_enabled"`
		SecureBootCertificates          []SystemSecuritySecureBootCertificate `json:"secure_boot_certificates"           yaml:"secure_boot_certificates"`
	} `json:"state" yaml:"state"`
}

// SystemSecuritySecureBootCertificate defines a struct that holds information about Secure Boot keys present on the host.
type SystemSecuritySecureBootCertificate struct {
	Type        string `json:"type"        yaml:"type"`
	Fingerprint string `json:"fingerprint" yaml:"fingerprint"`
	Subject     string `json:"subject"     yaml:"subject"`
	Issuer      string `json:"issuer"      yaml:"issuer"`
}
