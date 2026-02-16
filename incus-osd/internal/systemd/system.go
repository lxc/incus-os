package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// SystemPowerOff triggers a system shutdown.
func SystemPowerOff(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemctl", "poweroff")
	if err != nil {
		return err
	}

	return nil
}

// SystemReboot triggers a system reboot.
func SystemReboot(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemctl", "reboot")
	if err != nil {
		return err
	}

	return nil
}

// SystemSuspend triggers a system suspend.
func SystemSuspend(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemctl", "suspend")
	if err != nil {
		return err
	}

	return nil
}
