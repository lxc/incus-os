package api

import (
	syncconfig "github.com/FuturFusion/openfga-sync/shared/config"
)

// ApplicationOpenFGAConfig represents additional configuration for the openfga application.
type ApplicationOpenFGAConfig struct {
	ApplicationConfig

	APITokens []string `json:"api_tokens" yaml:"api_tokens"`

	// Sync holds the openfga-sync configuration, the daemon only runs when set.
	Sync *syncconfig.Config `incusos:"-" json:"sync,omitempty" yaml:"sync,omitempty"`
}

// ApplicationOpenFGAState represents the state of the openfga application.
type ApplicationOpenFGAState struct {
	ApplicationState
}

// ApplicationOpenFGA represents the state and configuration of the openfga application.
type ApplicationOpenFGA struct {
	State ApplicationOpenFGAState `json:"state" yaml:"state"`

	Config ApplicationOpenFGAConfig `json:"config" yaml:"config"`
}
