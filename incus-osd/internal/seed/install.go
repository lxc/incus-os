package seed

import (
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetInstall extracts the installation config from the seed data.
func GetInstall(partition string) (*apiseed.Install, error) {
	// Get the install configuration.
	var config apiseed.Install

	err := parseFileContents(partition, "install", &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
