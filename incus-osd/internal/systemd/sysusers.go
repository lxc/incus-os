package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

func RefreshUsers(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysusers")
	if err != nil {
		return err
	}

	return nil
}
