package api

import (
	"time"
)

// ApplicationConfig represents additional configuration for a generic application.
type ApplicationConfig struct{}

// ApplicationState represents the state of a generic application.
type ApplicationState struct {
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
type ApplicationAction struct {
	Action string            `json:"action"           yaml:"action"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}
