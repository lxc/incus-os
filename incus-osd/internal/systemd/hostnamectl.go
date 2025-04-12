package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// SetHostname sets the system's hostname to the provided value.
func SetHostname(ctx context.Context, hostname string) error {
	_, err := subprocess.RunCommandContext(ctx, "hostnamectl", "hostname", hostname)
	if err != nil {
		return err
	}

	return nil
}
