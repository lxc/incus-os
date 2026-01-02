package api

import (
	"time"
)

// ApplicationConfig represents additional configuration for an application.
type ApplicationConfig struct{}

// ApplicationState represents the state of the application.
type ApplicationState struct {
	Initialized  bool       `json:"initialized"             yaml:"initialized"`
	Version      string     `json:"version"                 yaml:"version"`
	LastRestored *time.Time `json:"last_restored,omitempty" yaml:"last_restored,omitempty"` // In system's timezone.
}

// Application represents the state and configuration of a generic application.
type Application struct {
	State ApplicationState `json:"state" yaml:"state"`

	Config ApplicationConfig `json:"config" yaml:"config"`
}
