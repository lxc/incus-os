package api

// SystemEncryption defines a struct to hold information about the system's encryption state.
type SystemEncryption struct {
	RecoveryKeys          []string `json:"recovery_keys"`
	RecoveryKeysRetrieved bool     `json:"recovery_keys_retrieved"`
}
