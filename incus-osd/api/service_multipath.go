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

// ServiceMultipathController represents a single Fibre Channel controller.
type ServiceMultipathController struct {
	FabricName      string `json:"fabric_name"      yaml:"fabric_name"`
	NodeName        string `json:"node_name"        yaml:"node_name"`
	PortName        string `json:"port_name"        yaml:"port_name"`
	PortState       string `json:"port_state"       yaml:"port_state"`
	PortType        string `json:"port_type"        yaml:"port_type"`
	Speed           string `json:"speed"            yaml:"speed"`
	SupportedSpeeds string `json:"supported_speeds" yaml:"supported_speeds"`
}

// ServiceMultipathConfig represents additional configuration for the Multipath service.
type ServiceMultipathConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	WWNs    []string `json:"wwns"    yaml:"wwns"`
}

// ServiceMultipath represents the state and configuration of the Multipath service.
type ServiceMultipath struct {
	State ServiceMultipathState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceMultipathConfig `json:"config" yaml:"config"`
}

// ServiceMultipathState represents the state for the Multipath service.
type ServiceMultipathState struct {
	Controllers []ServiceMultipathController      `json:"controllers" yaml:"controllers"`
	Devices     map[string]ServiceMultipathDevice `json:"devices"     yaml:"devices"`
}
