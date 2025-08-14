package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetMigrationManager extracts the Operations Center preseed from the seed data.
func GetMigrationManager(_ context.Context, partition string) (*apiseed.MigrationManager, error) {
	// Get the preseed.
	var preseed apiseed.MigrationManager

	err := parseFileContents(partition, "migration-manager", &preseed)
	if err != nil {
		return nil, err
	}

	return &preseed, nil
}
