package api

// SystemSecurityState holds information about the current security state.
type SystemSecurityState struct {
	EncryptionRecoveryKeysRetrieved bool                                  `json:"encryption_recovery_keys_retrieved" yaml:"encryption_recovery_keys_retrieved"`
	EncryptedVolumes                []SystemSecurityEncryptedVolume       `incusos:"-"                               json:"encrypted_volumes"                  yaml:"encrypted_volumes"`
	SecureBootEnabled               bool                                  `incusos:"-"                               json:"secure_boot_enabled"                yaml:"secure_boot_enabled"`
	SecureBootCertificates          []SystemSecuritySecureBootCertificate `incusos:"-"                               json:"secure_boot_certificates"           yaml:"secure_boot_certificates"`
	TPMStatus                       string                                `incusos:"-"                               json:"tpm_status"                         yaml:"tpm_status"`
	PoolRecoveryKeys                map[string]string                     `incusos:"-"                               json:"pool_recovery_keys"                 yaml:"pool_recovery_keys"`
	SystemStateIsTrusted            bool                                  `incusos:"-"                               json:"system_state_is_trusted"            yaml:"system_state_is_trusted"`
}

// SystemSecurityConfig holds additional security configuration settings.
type SystemSecurityConfig struct {
	EncryptionRecoveryKeys []string `json:"encryption_recovery_keys" yaml:"encryption_recovery_keys"`
}

// SystemSecurity defines a struct to hold information about the system's security state.
type SystemSecurity struct {
	Config SystemSecurityConfig `json:"config" yaml:"config"`

	State SystemSecurityState `json:"state" yaml:"state"`
}

// SystemSecuritySecureBootCertificate defines a struct that holds information about Secure Boot keys present on the host.
type SystemSecuritySecureBootCertificate struct {
	Type        string `json:"type"        yaml:"type"`
	Fingerprint string `json:"fingerprint" yaml:"fingerprint"`
	Subject     string `json:"subject"     yaml:"subject"`
	Issuer      string `json:"issuer"      yaml:"issuer"`
}

// SystemSecurityEncryptedVolume defines a struct that holds basic information about an encrypted volume.
type SystemSecurityEncryptedVolume struct {
	Volume string `json:"volume" yaml:"volume"`
	State  string `json:"state"  yaml:"state"`
}
