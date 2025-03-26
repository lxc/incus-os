package seed

import (
	"context"
)

// GetNetwork extracts the network configuration from the seed data.
func GetNetwork(_ context.Context, partition string) (*NetworkConfig, error) {
	// Get the network configuration.
	var config NetworkConfig

	err := parseFileContents(partition, "network", &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
