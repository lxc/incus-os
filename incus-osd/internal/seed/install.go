package seed

// InstallConfig defines a struct to hold install configuration.
type InstallConfig struct {
	ForceInstall        bool   `json:"force_install"         yaml:"forceInstall"`
	TargetDiskSubstring string `json:"target_disk_substring" yaml:"targetDiskSubstring"`
}

// GetInstallConfig extracts the list of applications from the seed data.
func GetInstallConfig(partition string) (*InstallConfig, error) {
	// Get the install configuration.
	var config InstallConfig

	err := parseFileContents(partition, "install", &config)
	if err != nil {
		return &InstallConfig{}, err
	}

	return &config, nil
}
