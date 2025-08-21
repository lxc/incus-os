package api

// Application represents the state and configuration of a generic application.
type Application struct {
	State struct {
		Initialized bool   `json:"initialized" yaml:"initialized"`
		Version     string `json:"version"     yaml:"version"`
	} `json:"state" yaml:"state"`

	Config struct{} `json:"config" yaml:"config"`
}
