package api

// SystemSecurity defines a struct to hold information about the system's security state.
type SystemSecurity struct {
	Config struct {
		EncryptionRecoveryKeys []string `json:"encryption_recovery_keys" yaml:"encryption_recovery_keys"`
	} `json:"config" yaml:"config"`

	State struct {
		EncryptionRecoveryKeysRetrieved bool `json:"encryption_recovery_keys_retrieved" yaml:"encryption_recovery_keys_retrieved"`
	} `json:"state" yaml:"state"`
}
