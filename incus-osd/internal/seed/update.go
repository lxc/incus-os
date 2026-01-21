package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetUpdate extracts the update configuration from the seed data.
func GetUpdate(_ context.Context) (*apiseed.Update, error) {
	// Get the update configuration.
	var config apiseed.Update

	err := parseFileContents(getSeedPath(), "update", &config)
	if err != nil {
		return nil, err
	}

	// Ensure the seed is valid.
	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
}
