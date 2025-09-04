package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetOperationsCenter extracts the Operations Center preseed from the seed data.
func GetOperationsCenter(_ context.Context, partition string) (*apiseed.OperationsCenter, error) {
	// Get the preseed.
	var preseed apiseed.OperationsCenter

	err := parseFileContents(partition, "operations-center", &preseed)
	if err != nil {
		return nil, err
	}

	return &preseed, nil
}
