package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

func RefreshExtensions(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysext", "refresh")
	if err != nil {
		return err
	}

	return nil
}
