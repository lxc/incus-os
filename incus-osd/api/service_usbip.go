package api

// ServiceUSBIPTarget represents a single USBIP target.
type ServiceUSBIPTarget struct {
	Address string `json:"address" yaml:"address"`
	BusID   string `json:"bus_id"  yaml:"bus_id"`
}

// ServiceUSBIP represents the state and configuration of the USBIP service.
type ServiceUSBIP struct {
	State struct{} `incusos:"-" json:"state" yaml:"state"`

	Config struct {
		Targets []ServiceUSBIPTarget `json:"targets" yaml:"targets"`
	} `json:"config" yaml:"config"`
}
