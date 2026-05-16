package seed

import (
	"context"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetKernel extracts the kernel configuration from the seed data.
func GetKernel(_ context.Context) (*apiseed.Kernel, error) {
	// Get the kernel configuration.
	var config apiseed.Kernel

	err := parseFileContents(getSeedPath(), "kernel", &config)
	if err != nil && !IsMissing(err) {
		return nil, err
	}

	return &config, nil
}
