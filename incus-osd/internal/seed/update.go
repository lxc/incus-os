package seed

import (
	"context"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetUpdate extracts the update configuration from the seed data.
func GetUpdate(_ context.Context, existingConfig *api.SystemUpdateConfig) (*apiseed.Update, error) {
	// Get the update configuration.
	var config apiseed.Update

	err := parseFileContents(getSeedPath(), "update", &config)
	if err != nil {
		return nil, err
	}

	// If an existing configuration was provided and the seed is missing either
	// the Channel or CheckFrequency fields, copy the existing values. We can't
	// differentiate an update or missing values for AutoReboot and
	// MaintenanceWindows, so those will always overwrite any previously set value(s).
	if existingConfig != nil {
		if config.Channel == "" {
			config.Channel = existingConfig.Channel
		}

		if config.CheckFrequency == "" {
			config.CheckFrequency = existingConfig.CheckFrequency
		}
	}

	// Ensure the seed is valid.
	err = config.Validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
}
