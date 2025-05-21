package seed

import (
	"context"

	incusapi "github.com/lxc/incus/v6/shared/api"
)

// IncusConfig is a wrapper around the Incus preseed.
type IncusConfig struct {
	Version string `json:"version" yaml:"version"`

	ApplyDefaults bool `json:"apply_defaults" yaml:"apply_defaults"`

	Preseed *incusapi.InitPreseed `json:"preseed" yaml:"preseed"`
}

// GetIncus extracts the Incus preseed from the seed data.
func GetIncus(_ context.Context, partition string) (*IncusConfig, error) {
	// Get the preseed.
	var preseed IncusConfig

	err := parseFileContents(partition, "incus", &preseed)
	if err != nil {
		return nil, err
	}

	return &preseed, nil
}
