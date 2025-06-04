package seed

import (
	"context"

	"github.com/lxc/incus-os/incus-osd/api"
)

// ProviderSeed defines a struct to hold provider configuration.
type ProviderSeed struct {
	api.SystemProviderConfig `yaml:",inline"`

	Version string `json:"version" yaml:"version"`
}

// GetProvider extracts the provider configuration from the seed data.
func GetProvider(_ context.Context, partition string) (*ProviderSeed, error) {
	// Get the install configuration.
	var config ProviderSeed

	err := parseFileContents(partition, "provider", &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
