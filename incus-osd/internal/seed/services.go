package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetServices extracts the services configuration from the seed data.
func GetServices(_ context.Context) (*apiseed.Services, error) {
	// Get the services configuration.
	var config apiseed.Services

	err := parseFileContents(getSeedPath(), "services", &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
