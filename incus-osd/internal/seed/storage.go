package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetStorage extracts the storage configuration from the seed data.
func GetStorage(_ context.Context) (*apiseed.Storage, error) {
	// Get the storage configuration.
	var config apiseed.Storage

	err := parseFileContents(getSeedPath(), "storage", &config)
	if err != nil && !IsMissing(err) {
		return nil, err
	}

	return &config, nil
}
