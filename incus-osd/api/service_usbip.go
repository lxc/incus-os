package api

// ServiceUSBIPTarget represents a single USBIP target.
type ServiceUSBIPTarget struct {
	Address string `json:"address" yaml:"address"`
	BusID   string `json:"bus_id"  yaml:"bus_id"`
}

// ServiceUSBIPConfig represents additional configuration for the USBIP service.
type ServiceUSBIPConfig struct {
	Targets []ServiceUSBIPTarget `json:"targets" yaml:"targets"`
}

// ServiceUSBIPState represents state for the USBIP service.
type ServiceUSBIPState struct{}

// ServiceUSBIP represents the state and configuration of the USBIP service.
type ServiceUSBIP struct {
	State ServiceUSBIPState `incusos:"-" json:"state" yaml:"state"`

	Config ServiceUSBIPConfig `json:"config" yaml:"config"`
}
