package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// ReloadDaemon instructs systemd to reload all its units.
func ReloadDaemon(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemctl", "daemon-reload")
	if err != nil {
		return err
	}

	return nil
}
