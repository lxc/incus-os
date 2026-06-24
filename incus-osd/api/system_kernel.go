package api

// SystemKernelConfig holds the kernel-level configuration data.
type SystemKernelConfig struct {
	Console          []SystemKernelConfigConsole `json:"console,omitempty"           yaml:"console,omitempty"`
	BlacklistModules []string                    `json:"blacklist_modules,omitempty" yaml:"blacklist_modules,omitempty"`
	Memory           *SystemKernelConfigMemory   `json:"memory,omitempty"            yaml:"memory,omitempty"`
	Network          *SystemKernelConfigNetwork  `json:"network,omitempty"           yaml:"network,omitempty"`
	PCI              *SystemKernelConfigPCI      `json:"pci,omitempty"               yaml:"pci,omitempty"`
}

// SystemKernelConfigConsole holds console-specific kernel configuration.
type SystemKernelConfigConsole struct {
	Device   string `json:"device"              yaml:"device"`
	BaudRate int    `json:"baud_rate,omitempty" yaml:"baud_rate,omitempty"`
}

// SystemKernelConfigMemory holds memory-specific kernel configuration.
type SystemKernelConfigMemory struct {
	PersistentHugepages int    `json:"persistent_hugepages" yaml:"persistent_hugepages"`
	ZramSwapSize        string `json:"zram_swap_size"       yaml:"zram_swap_size"`
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
type SystemKernelState struct {
	Memory *SystemKernelStateMemory `json:"memory,omitempty" yaml:"memory,omitempty"`
}

// SystemKernelStateMemory represents state for the system's kernel-level memory configuration.
type SystemKernelStateMemory struct {
	ZramSwap *SystemKernelStateMemoryZramSwap `json:"zram_swap,omitempty" yaml:"zram_swap,omitempty"`
}

// SystemKernelStateMemoryZramSwap reports statistics about the configured zram-backed swap device, if present.
// Reported sizes are in bytes.
type SystemKernelStateMemoryZramSwap struct {
	Disksize         int     `json:"disk_size"         yaml:"disk_size"`
	UncompressedSize int     `json:"incompressed_size" yaml:"uncompressed_size"`
	CompressedSize   int     `json:"compressed_size"   yaml:"compressed_size"`
	CompressionRatio float64 `json:"compression_ratio" yaml:"compression_ratio"`
	TotalMemoryUse   int     `json:"total_memory_use"  yaml:"total_memory_use"`
}

// SystemKernel defines a struct to hold information about the system's kernel-level configuration.
type SystemKernel struct {
	Config SystemKernelConfig `json:"config" yaml:"config"`
	State  SystemKernelState  `incusos:"-"   json:"state"  yaml:"state"`
}
