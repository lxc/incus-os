package api

// DebugKernelModule represents a loaded kernel module.
type DebugKernelModule struct {
	Name         string   `json:"name"         yaml:"name"`
	Dependencies []string `json:"dependencies" yaml:"dependencies"`
	InUse        bool     `json:"in_use"       yaml:"in_use"`
}

// DebugKernel represents kernel debug information for the system.
//
// swagger:model
type DebugKernel struct {
	Version      string              `json:"version"      yaml:"version"`
	Architecture string              `json:"architecture" yaml:"architecture"`
	CPUBaseline  string              `json:"cpu_baseline" yaml:"cpu_baseline"`
	Modules      []DebugKernelModule `json:"modules"      yaml:"modules"`
}
