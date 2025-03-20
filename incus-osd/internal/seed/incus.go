package seed

import (
	"context"

	incusapi "github.com/lxc/incus/v6/shared/api"
)

// GetIncus extracts the Incus preseed from the seed data.
func GetIncus(_ context.Context, partition string) (*incusapi.InitPreseed, error) {
	// Get the preseed.
	var preseed incusapi.InitPreseed

	err := parseFileContents(partition, "incus", &preseed)
	if err != nil {
		return nil, err
	}

	return &preseed, nil
}
