package seed

import (
	"context"
	"errors"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetSecurity extracts the security configuration from the seed data.
func GetSecurity(_ context.Context) (*apiseed.Security, error) {
	// Get the security configuration.
	var config apiseed.Security

	err := parseFileContents(getSeedPath(), "security", &config)
	if err != nil {
		return nil, err
	}

	if len(config.EncryptionRecoveryKeys) != 0 {
		return nil, errors.New("it is not possible to set encryption recovery key(s) via the security seed")
	}

	return &config, nil
}
