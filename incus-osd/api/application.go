package api

import (
	"time"
)

// Application represents the state and configuration of a generic application.
type Application struct {
	State struct {
		Initialized  bool       `json:"initialized"             yaml:"initialized"`
		Version      string     `json:"version"                 yaml:"version"`
		LastRestored *time.Time `json:"last_restored,omitempty" yaml:"last_restored,omitempty"`
	} `json:"state" yaml:"state"`

	Config struct{} `json:"config" yaml:"config"`
}
