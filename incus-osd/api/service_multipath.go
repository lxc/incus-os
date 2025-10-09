package api

// ServiceMultipathDevice represents a single Multipath device.
type ServiceMultipathDevice struct {
	Vendor     string                      `json:"vendor"      yaml:"vendor"`
	Size       string                      `json:"size"        yaml:"size"`
	PathGroups []ServiceMultipathPathGroup `json:"path_groups" yaml:"path_groups"`
}

// ServiceMultipathPathGroup represents a single Multipath path group.
type ServiceMultipathPathGroup struct {
	Policy   string                 `json:"policy"   yaml:"policy"`
	Priority uint64                 `json:"priority" yaml:"priority"`
	Status   string                 `json:"status"   yaml:"status"`
	Paths    []ServiceMultipathPath `json:"paths"    yaml:"paths"`
}

// ServiceMultipathPath represents a single Multipath path.
type ServiceMultipathPath struct {
	ID     string `json:"id"     yaml:"id"`
	Status string `json:"status" yaml:"status"`
}

// ServiceMultipath represents the state and configuration of the Multipath service.
type ServiceMultipath struct {
	State ServiceMultipathState `json:"state" yaml:"state"`

	Config struct {
		Enabled bool     `json:"enabled" yaml:"enabled"`
		WWNs    []string `json:"wwns"    yaml:"wwns"`
	} `json:"config" yaml:"config"`
}

// ServiceMultipathState represents the state for the Multipath service.
type ServiceMultipathState struct {
	Devices map[string]ServiceMultipathDevice `json:"devices" yaml:"devices"`
}
