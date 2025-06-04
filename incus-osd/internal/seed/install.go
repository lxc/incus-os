package seed

// InstallSeed defines a struct to hold install configuration.
type InstallSeed struct {
	Version string `json:"version" yaml:"version"`

	ForceInstall bool               `json:"force_install" yaml:"force_install"` // If true, ignore any existing data on target install disk.
	ForceReboot  bool               `json:"force_reboot"  yaml:"force_reboot"`  // If true, reboot the system automatically upon completion rather than waiting for the install media to be removed.
	Target       *InstallSeedTarget `json:"target"        yaml:"target"`        // Optional selector for the target install disk; if not set, expect a single drive to be present.
}

// InstallSeedTarget defines options used to select the target install disk.
type InstallSeedTarget struct {
	ID string `json:"id" yaml:"id"` // Name as listed in /dev/disk/by-id/, glob supported.
}

// GetInstallConfig extracts the installation config from the seed data.
func GetInstallConfig(partition string) (*InstallSeed, error) {
	// Get the install configuration.
	var config InstallSeed

	err := parseFileContents(partition, "install", &config)
	if err != nil {
		return &InstallSeed{}, err
	}

	return &config, nil
}
