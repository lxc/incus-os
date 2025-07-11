package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetIncus extracts the Incus preseed from the seed data.
func GetIncus(_ context.Context, partition string) (*apiseed.Incus, error) {
	// Get the preseed.
	var preseed apiseed.Incus

	err := parseFileContents(partition, "incus", &preseed)
	if err != nil {
		return nil, err
	}

	return &preseed, nil
}
