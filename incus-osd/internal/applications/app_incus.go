package applications

import (
	"context"

	incusclient "github.com/lxc/incus/v6/client"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

type incus struct{}

// Start starts all the systemd units.
func (*incus) Start(ctx context.Context, _ string) error {
	return systemd.EnableUnit(ctx, true, "incus.socket", "incus-lxcfs.service", "incus-startup.service", "incus.service")
}

// Initialize runs first time initialization.
func (*incus) Initialize(ctx context.Context) error {
	// Get the preseed from the seed partition.
	incusPreseed, err := seed.GetIncus(ctx, seed.SeedPartitionPath)
	if err != nil && !seed.IsMissing(err) {
		return err
	}

	// If no preseed, we're done.
	if incusPreseed == nil {
		return nil
	}

	// Connect to Incus.
	c, err := incusclient.ConnectIncusUnix("", nil)
	if err != nil {
		return err
	}

	// Push the preseed.
	err = c.ApplyServerPreseed(*incusPreseed)
	if err != nil {
		return err
	}

	return nil
}
