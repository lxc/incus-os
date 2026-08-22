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

	Services ApplicationIncusStateServices `incusos:"-" json:"services" yaml:"services"`
}

// ApplicationIncusStateServices represents the state of the services managed through the Incus application.
type ApplicationIncusStateServices struct {
	Ceph ApplicationIncusStateServicesCeph `json:"ceph" yaml:"ceph"`
}

// ApplicationIncusStateServicesCeph represents the state of a managed Ceph cluster.
type ApplicationIncusStateServicesCeph struct {
	Deployed bool                                   `json:"deployed"          yaml:"deployed"`
	Version  string                                 `json:"version,omitempty" yaml:"version,omitempty"`
	FSID     string                                 `json:"fsid,omitempty"    yaml:"fsid,omitempty"`
	OSDs     []ApplicationIncusStateServicesCephOSD `json:"osds,omitempty"    yaml:"osds,omitempty"`
}

// ApplicationIncusStateServicesCephOSD represents a single OSD of a managed Ceph cluster.
type ApplicationIncusStateServicesCephOSD struct {
	Host   string `json:"host"   yaml:"host"`
	Device string `json:"device" yaml:"device"`
}

// ApplicationIncus represents the state and configuration of the Incus application.
type ApplicationIncus struct {
	State ApplicationIncusState `json:"state" yaml:"state"`

	Config ApplicationIncusConfig `json:"config" yaml:"config"`
}
