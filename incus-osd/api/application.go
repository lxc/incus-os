package api

import (
	"time"
)

// ApplicationConfig represents additional configuration for a generic application.
type ApplicationConfig struct {
	PinCurrentVersion bool `json:"pin_current_version" yaml:"pin_current_version"`
}

// ApplicationState represents the state of a generic application.
type ApplicationState struct {
	IsPrimary         bool       `incusos:"-"                    json:"is_primary"              yaml:"is_primary"`
	Initialized       bool       `json:"initialized"             yaml:"initialized"`
	FriendlyVersion   string     `json:"friendly_version"        yaml:"friendly_version"`
	Version           string     `json:"version"                 yaml:"version"`
	AvailableVersions []string   `json:"available_versions"      yaml:"available_versions"`
	LastRestored      *time.Time `json:"last_restored,omitempty" yaml:"last_restored,omitempty"` // In system's timezone.
}

// Application represents the state and configuration of a generic application.
type Application struct {
	State ApplicationState `json:"state" yaml:"state"`

	Config ApplicationConfig `json:"config" yaml:"config"`
}

// ApplicationAction defines a generic struct that can be used when triggering an application-specific action.
//
// swagger:model
type ApplicationAction struct {
	Action string            `json:"action"           yaml:"action"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}
