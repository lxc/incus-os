package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

func EnableUnit(ctx context.Context, now bool, units ...string) error {
	args := []string{"enable"}

	if now {
		args = append(args, "--now")
	}

	args = append(args, units...)

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}
