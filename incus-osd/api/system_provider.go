package api

// SystemProviderConfig holds the modifiable part of the provider data.
type SystemProviderConfig struct {
	Name   string            `json:"name"             yaml:"name"`
	Config map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

// SystemProviderState holds information about the current provider state.
type SystemProviderState struct {
	Registered bool `json:"registered" yaml:"registered"`
}

// SystemProvider defines a struct to hold information about the system's update and configuration provider.
type SystemProvider struct {
	Config SystemProviderConfig `json:"config" yaml:"config"`
	State  SystemProviderState  `json:"state"  yaml:"state"`
}
