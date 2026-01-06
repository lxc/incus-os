package api

// SystemKernelConfig holds the kernel-level configuration data.
type SystemKernelConfig struct {
	BlacklistModules []string                   `json:"blacklist_modules,omitempty" yaml:"blacklist_modules,omitempty"`
	Network          *SystemKernelConfigNetwork `json:"network,omitempty"           yaml:"network,omitempty"`
	PCI              *SystemKernelConfigPCI     `json:"pci,omitempty"               yaml:"pci,omitempty"`
}

// SystemKernelConfigNetwork holds network-specific kernel configuration.
type SystemKernelConfigNetwork struct {
	BufferSize             int    `json:"buffer_size,omitempty"              yaml:"buffer_size,omitempty"`
	QueuingDiscipline      string `json:"queuing_discipline,omitempty"       yaml:"queuing_discipline,omitempty"`
	TCPCongestionAlgorithm string `json:"tcp_congestion_algorithm,omitempty" yaml:"tcp_congestion_algorithm,omitempty"`
}

// SystemKernelConfigPCI holds PCI-specific kernel configuration.
type SystemKernelConfigPCI struct {
	Passthrough []SystemKernelConfigPCIPassthrough `json:"passthrough,omitempty" yaml:"passthrough,omitempty"`
}

// SystemKernelConfigPCIPassthrough defines a specific PCI device that should be made available for passthrough to a VM.
type SystemKernelConfigPCIPassthrough struct {
	VendorID   string `json:"vendor_id"             yaml:"vendor_id"`
	ProductID  string `json:"product_id"            yaml:"product_id"`
	PCIAddress string `json:"pci_address,omitempty" yaml:"pci_address,omitempty"`
}

// SystemKernelState represents state for the system's kernel-level configuration.
type SystemKernelState struct{}

// SystemKernel defines a struct to hold information about the system's kernel-level configuration.
type SystemKernel struct {
	Config SystemKernelConfig `json:"config" yaml:"config"`
	State  SystemKernelState  `incusos:"-"   json:"state"  yaml:"state"`
}
