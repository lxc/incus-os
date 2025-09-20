package api

// Application represents the state and configuration of a generic application.
type Application struct {
	State struct {
		Initialized bool   `json:"initialized" yaml:"initialized"`
		Version     string `json:"version"     yaml:"version"`
	} `json:"state" yaml:"state"`

	Config struct{} `json:"config" yaml:"config"`
}

// ApplicationDelete defines a struct with parameters used when uninstalling an application.
type ApplicationDelete struct {
	RemoveUserData bool `json:"remove_user_data" yaml:"remove_user_data"`
}
