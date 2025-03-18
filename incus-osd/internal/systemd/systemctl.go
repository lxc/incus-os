package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// StartUnit instructs systemd to start the provided unit(s).
func StartUnit(ctx context.Context, units ...string) error {
	args := []string{"start"}
	args = append(args, units...)

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}

// RestartUnit instructs systemd to restart the provided unit(s).
func RestartUnit(ctx context.Context, units ...string) error {
	args := []string{"restart"}
	args = append(args, units...)

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}

// EnableUnit instructs systemd to enable (and optionally start) the provided unit(s).
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
