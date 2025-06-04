package api

// SystemProviderConfig holds the modifiable part of the provider data.
type SystemProviderConfig struct {
	Name   string            `json:"name"   yaml:"name"`
	Config map[string]string `json:"config" yaml:"config"`
}

// SystemProvider defines a struct to hold information about the system's update and configuration provider.
type SystemProvider struct {
	Config SystemProviderConfig `json:"config" yaml:"config"`
	State  struct{}             `json:"state"  yaml:"state"`
}
