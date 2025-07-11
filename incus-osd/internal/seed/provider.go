package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetProvider extracts the provider configuration from the seed data.
func GetProvider(_ context.Context, partition string) (*apiseed.Provider, error) {
	// Get the install configuration.
	var config apiseed.Provider

	err := parseFileContents(partition, "provider", &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
