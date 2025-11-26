package seed

import (
	"errors"
	"io"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetInstall extracts the installation config from the seed data.
func GetInstall() (*apiseed.Install, error) {
	// Get the install configuration.
	var config apiseed.Install

	err := parseFileContents(getSeedPath(), "install", &config)
	if err != nil {
		// If we have any empty install file, that should still trigger an install.
		if errors.Is(err, io.EOF) {
			return &config, nil
		}

		return nil, err
	}

	return &config, nil
}
