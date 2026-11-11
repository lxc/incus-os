package api

// SystemPowerConfig holds power management related configuration for the system.
type SystemPowerConfig struct {
	CPU *SystemPowerConfigCPU `json:"cpu,omitempty" yaml:"cpu,omitempty"`
}

// SystemPowerConfigCPU holds power management related configuration for CPU.
type SystemPowerConfigCPU struct {
	ScalingGovernor             string `json:"scaling_governor,omitempty"              yaml:"scaling_governor,omitempty"`
	EnergyPerformancePreference string `json:"energy_performance_preference,omitempty" yaml:"energy_performance_preference,omitempty"`
}

// SystemPowerState represents state for the system's power management configuration.
type SystemPowerState struct {
	CPU SystemPowerStateCPU `json:"cpu" yaml:"cpu"`
}

// SystemPowerStateCPU represents state for the system's CPU related power management configuration.
type SystemPowerStateCPU struct {
	ScalingDriver                         string   `json:"scaling_driver"                           yaml:"scaling_driver"`
	ScalingGovernor                       string   `json:"scaling_governor"                         yaml:"scaling_governor"`
	ScalingAvailableGovernors             []string `json:"scaling_available_governors"              yaml:"scaling_available_governors"`
	EnergyPerformancePreference           string   `json:"energy_performance_preference"            yaml:"energy_performance_preference"`
	EnergyPerformanceAvailablePreferences []string `json:"energy_performance_available_preferences" yaml:"energy_performance_available_preferences"`
}

// SystemPower defines a struct to hold information about the system's power management configuration and its state.
//
// swagger:model
type SystemPower struct {
	Config SystemPowerConfig `json:"config" yaml:"config"`
	State  SystemPowerState  `incusos:"-"   json:"state"  yaml:"state"`
}
