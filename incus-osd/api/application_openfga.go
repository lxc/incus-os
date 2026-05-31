package api

// ApplicationOpenFGAConfig represents additional configuration for the openfga application.
type ApplicationOpenFGAConfig struct {
	ApplicationConfig

	APITokens []string `json:"api_tokens" yaml:"api_tokens"`
}

// ApplicationOpenFGAState represents the state of the openfga application.
type ApplicationOpenFGAState struct {
	ApplicationState

	StoreID string `json:"store_id" yaml:"store_id"`
}

// ApplicationOpenFGA represents the state and configuration of the openfga application.
type ApplicationOpenFGA struct {
	State ApplicationOpenFGAState `json:"state" yaml:"state"`

	Config ApplicationOpenFGAConfig `json:"config" yaml:"config"`
}
