package api

// ApplicationIncusConfigLXCFS represents the LXCFS configuration options for the Incus application.
type ApplicationIncusConfigLXCFS struct {
	CPUShares   bool `json:"cpu_shares"   yaml:"cpu_shares"`
	LoadAverage bool `json:"load_average" yaml:"load_average"`
}

// ApplicationIncusConfig represents additional configuration for the Incus application.
type ApplicationIncusConfig struct {
	ApplicationConfig

	LXCFS ApplicationIncusConfigLXCFS `json:"lxcfs" yaml:"lxcfs"`
}

// ApplicationIncusState represents the state of the Incus application.
type ApplicationIncusState struct {
	ApplicationState
}

// ApplicationIncus represents the state and configuration of the Incus application.
type ApplicationIncus struct {
	State ApplicationIncusState `json:"state" yaml:"state"`

	Config ApplicationIncusConfig `json:"config" yaml:"config"`
}
