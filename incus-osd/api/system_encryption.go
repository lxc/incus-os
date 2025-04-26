package api

// SystemEncryption defines a struct to hold information about the system's encryption state.
type SystemEncryption struct {
	Config struct {
		RecoveryKeys []string `json:"recovery_keys" yaml:"recovery_keys"`
	} `json:"config" yaml:"config"`

	State struct {
		RecoveryKeysRetrieved bool `json:"recovery_keys_retrieved" yaml:"recovery_keys_retrieved"`
	} `json:"state" yaml:"state"`
}
