package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// RefreshUsers instructs systemd-sysusers to re-scan and re-apply user definitions.
func RefreshUsers(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysusers")
	if err != nil {
		return err
	}

	return nil
}
