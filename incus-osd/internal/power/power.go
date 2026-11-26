package power

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/lxc/incus/v7/shared/revert"

	"github.com/lxc/incus-os/incus-osd/api"
)

const (
	cpu0FreqPath     = "/sys/devices/system/cpu/cpu0/cpufreq/"
	noTurboPath      = "/sys/devices/system/cpu/intel_pstate/no_turbo"
	cpufreqBoostPath = "/sys/devices/system/cpu/cpufreq/boost"
)

// frequencyBoostingControl describes a sysfs file controlling CPU frequency boosting and its enabled/disabled values.
type frequencyBoostingControl struct {
	path          string
	disabledValue string
	enabledValue  string
}

// frequencyBoostingControls lists candidate frequencyBoostingControl on the system in order of preference.
var frequencyBoostingControls = []frequencyBoostingControl{
	{path: noTurboPath, disabledValue: "1", enabledValue: "0"},
	{path: cpufreqBoostPath, disabledValue: "0", enabledValue: "1"},
}

// ErrInvalidConfig is returned when the configuration doesn't pass the validation check.
var ErrInvalidConfig = errors.New("invalid power configuration")

// ApplyPowerConfiguration sets power management related configuration for the system.
func ApplyPowerConfiguration(ctx context.Context, config api.SystemPowerConfig) error {
	// Validate the configuration.
	err := validatePowerConfiguration(config)
	if err != nil {
		return err
	}

	// Revert the configuration if this call fails.
	reverter := revert.New()
	defer reverter.Fail()

	// Get the current power management state before applying the configuration.
	currentState, err := GetPowerState(ctx)
	if err != nil {
		return err
	}

	// Only touch the blank timeout when it changes, as setting it wakes the screen.
	screenConfigChanged := config.Screen != nil && config.Screen.BlankTimeout != currentState.Screen.BlankTimeout

	// Revert the governor before epp as it controls what epp is valid.
	reverter.Add(func() {
		_ = applyCPUScalingGovernor(currentState.CPU.ScalingGovernor)
		_ = applyCPUEnergyPerformancePreference(currentState.CPU.EnergyPerformancePreference)

		if screenConfigChanged {
			_ = applyScreenBlankTimeout(currentState.Screen.BlankTimeout)
		}

		if currentState.CPU.FrequencyBoostingSupported {
			option := "enabled"
			if currentState.CPU.FrequencyBoostingDisabled {
				option = "disabled"
			}

			_ = applyCPUFrequencyBoosting(option)
		}
	})

	// Apply the power configuration for CPU.
	if config.CPU != nil {
		err := applyPowerConfigurationCPU(*config.CPU)
		if err != nil {
			return err
		}
	}

	// Apply the power configuration for the console screen.
	if screenConfigChanged {
		err := applyScreenBlankTimeout(config.Screen.BlankTimeout)
		if err != nil {
			return err
		}
	}

	// Clear revert functions.
	reverter.Success()

	return nil
}

// validatePowerConfiguration validates the power configuration.
func validatePowerConfiguration(config api.SystemPowerConfig) error {
	if config.CPU != nil {
		err := validatePowerConfigurationCPU(*config.CPU)
		if err != nil {
			return err
		}
	}

	if config.Screen != nil {
		err := validatePowerConfigurationScreen(*config.Screen)
		if err != nil {
			return err
		}
	}

	return nil
}

// validatePowerConfigurationCPU validates power configuration for CPU.
func validatePowerConfigurationCPU(config api.SystemPowerConfigCPU) error {
	err := validateSysfsString("scaling governor", config.ScalingGovernor, cpu0FreqPath+"scaling_available_governors")
	if err != nil {
		return err
	}

	err = validateSysfsString("energy performance preference", config.EnergyPerformancePreference, cpu0FreqPath+"energy_performance_available_preferences")
	if err != nil {
		return err
	}

	// Check against the running governor when the configuration doesn't set one.
	if config.ScalingGovernor == "" && config.EnergyPerformancePreference != "" {
		runningScalingGovernor, err := readSysfsString(cpu0FreqPath + "scaling_governor")
		if err != nil {
			return err
		}

		config.ScalingGovernor = runningScalingGovernor
	}

	// Energy performance preference can only be "performance" when scaling governor is "performance".
	if config.ScalingGovernor == "performance" && config.EnergyPerformancePreference != "" && config.EnergyPerformancePreference != "performance" {
		return fmt.Errorf("%w: energy performance preference %q can't be used with the \"performance\" scaling governor", ErrInvalidConfig, config.EnergyPerformancePreference)
	}

	// Check the frequency boosting value and whether the system supports it.
	if config.FrequencyBoosting != "" {
		if config.FrequencyBoosting != "enabled" && config.FrequencyBoosting != "disabled" {
			return fmt.Errorf("%w: frequency boosting must be \"enabled\" or \"disabled\"", ErrInvalidConfig)
		}

		_, ok, err := getFrequencyBoostingControl()
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("%w: frequency boosting is not supported on this system", ErrInvalidConfig)
		}
	}

	return nil
}

// applyPowerConfigurationCPU sets power management related configuration for the CPU of the system.
func applyPowerConfigurationCPU(config api.SystemPowerConfigCPU) error {
	// Apply CPU scaling governor config.
	err := applyCPUScalingGovernor(config.ScalingGovernor)
	if err != nil {
		return err
	}

	// Apply CPU energy performance preference config.
	err = applyCPUEnergyPerformancePreference(config.EnergyPerformancePreference)
	if err != nil {
		return err
	}

	// Apply frequency boosting config.
	err = applyCPUFrequencyBoosting(config.FrequencyBoosting)
	if err != nil {
		return err
	}

	return nil
}

// applyCPUScalingGovernor sets the provided scaling governor on all CPUs.
func applyCPUScalingGovernor(governor string) error {
	if governor == "" {
		return nil
	}

	// Apply the governor to every CPU.
	err := writeSysfs("/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor", governor)
	if err != nil {
		return err
	}

	return nil
}

// applyCPUEnergyPerformancePreference sets the provided energy performance preference on all CPUs.
func applyCPUEnergyPerformancePreference(epp string) error {
	if epp == "" {
		return nil
	}

	// Apply the EPP to every CPU.
	err := writeSysfs("/sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference", epp)
	if err != nil {
		if errors.Is(err, syscall.EBUSY) {
			return fmt.Errorf("energy performance preference %q can't be set while using the current scaling governor", epp)
		}

		return err
	}

	return nil
}

// applyCPUFrequencyBoosting enables or disables frequency boosting.
func applyCPUFrequencyBoosting(option string) error {
	var disable bool

	switch option {
	case "":
		return nil
	case "enabled":
		disable = false
	case "disabled":
		disable = true
	default:
		return fmt.Errorf("%w: frequency boosting must be \"enabled\" or \"disabled\"", ErrInvalidConfig)
	}

	control, ok, err := getFrequencyBoostingControl()
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("applying frequency boosting on system that does not support it")
	}

	if disable {
		err = writeSysfs(control.path, control.disabledValue)
	} else {
		err = writeSysfs(control.path, control.enabledValue)
	}

	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("%w: trying to enable frequency boosting while it is disabled on the firmware", ErrInvalidConfig)
		}

		return err
	}

	return nil
}

// GetPowerState returns current power management related settings from the running system.
func GetPowerState(_ context.Context) (api.SystemPowerState, error) {
	cpuState, err := getPowerStateCPU()
	if err != nil {
		return api.SystemPowerState{}, err
	}

	screenState, err := getPowerStateScreen()
	if err != nil {
		return api.SystemPowerState{}, err
	}

	return api.SystemPowerState{
		CPU:    cpuState,
		Screen: screenState,
	}, nil
}

// getPowerStateCPU returns current power management related state for CPU.
func getPowerStateCPU() (api.SystemPowerStateCPU, error) {
	scalingDriver, err := readSysfsString(cpu0FreqPath + "scaling_driver")
	if err != nil {
		return api.SystemPowerStateCPU{}, err
	}

	scalingGovernor, err := readSysfsString(cpu0FreqPath + "scaling_governor")
	if err != nil {
		return api.SystemPowerStateCPU{}, err
	}

	scalingAvailableGovernors, err := readSysfsList(cpu0FreqPath + "scaling_available_governors")
	if err != nil {
		return api.SystemPowerStateCPU{}, err
	}

	energyPerformancePreference, err := readSysfsString(cpu0FreqPath + "energy_performance_preference")
	if err != nil {
		return api.SystemPowerStateCPU{}, err
	}

	energyPerformanceAvailablePreferences, err := readSysfsList(cpu0FreqPath + "energy_performance_available_preferences")
	if err != nil {
		return api.SystemPowerStateCPU{}, err
	}

	var (
		frequencyBoostingSupported bool
		frequencyBoostingDisabled  bool
	)

	control, ok, err := getFrequencyBoostingControl()
	if err != nil {
		return api.SystemPowerStateCPU{}, err
	}

	if ok {
		frequencyBoostingSupported = true

		value, err := readSysfsString(control.path)
		if err != nil {
			return api.SystemPowerStateCPU{}, err
		}

		if value == control.disabledValue {
			frequencyBoostingDisabled = true
		}
	}

	return api.SystemPowerStateCPU{
		ScalingDriver:                         scalingDriver,
		ScalingGovernor:                       scalingGovernor,
		ScalingAvailableGovernors:             scalingAvailableGovernors,
		EnergyPerformancePreference:           energyPerformancePreference,
		EnergyPerformanceAvailablePreferences: energyPerformanceAvailablePreferences,
		FrequencyBoostingSupported:            frequencyBoostingSupported,
		FrequencyBoostingDisabled:             frequencyBoostingDisabled,
	}, nil
}

// readSysfsString returns whitespace trimmed content of a sysfs file, or an empty string if the path doesn't exist.
func readSysfsString(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

// readSysfsList returns a list of strings for a sysfs file, or empty if the path doesn't exist.
func readSysfsList(path string) ([]string, error) {
	content, err := readSysfsString(path)
	if err != nil {
		return nil, err
	}

	return strings.Fields(content), nil
}

// validateSysfsString returns an error if the provided value is not found in the sysfs path or there's no available value.
func validateSysfsString(name string, value string, path string) error {
	// Skip checking on unset/default value.
	if value == "" {
		return nil
	}

	available, err := readSysfsList(path)
	if err != nil {
		return err
	}

	if len(available) == 0 {
		return fmt.Errorf("%w: %s is not available on this system", ErrInvalidConfig, name)
	}

	if !slices.Contains(available, value) {
		return fmt.Errorf("%w: %s %q is not available (available: %s)", ErrInvalidConfig, name, value, strings.Join(available, ", "))
	}

	return nil
}

// writeSysfs writes the value to all matching paths based on the pattern.
func writeSysfs(pattern string, value string) error {
	entries, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		err := os.WriteFile(entry, []byte(value+"\n"), 0o644)
		if err != nil {
			return err
		}
	}

	return nil
}

// getFrequencyBoostingControl returns the frequency boosting control for the system and whether it's available.
func getFrequencyBoostingControl() (frequencyBoostingControl, bool, error) {
	for _, control := range frequencyBoostingControls {
		value, err := readSysfsString(control.path)
		if err != nil {
			return frequencyBoostingControl{}, false, err
		}

		if value != "" {
			return control, true, nil
		}
	}

	return frequencyBoostingControl{}, false, nil
}
