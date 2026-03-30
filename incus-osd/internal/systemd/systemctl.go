package systemd

import (
	"context"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// KillUnit kills the systemd unit(s) using the specified signal.
func KillUnit(ctx context.Context, signal string, units ...string) error {
	args := []string{"kill", "--signal=" + signal} //nolint:prealloc
	args = append(args, units...)

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}

// StartUnit instructs systemd to start the provided unit(s).
func StartUnit(ctx context.Context, units ...string) error {
	args := []string{"start"} //nolint:prealloc
	args = append(args, units...)

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}

// RestartUnit instructs systemd to restart the provided unit(s).
func RestartUnit(ctx context.Context, units ...string) error {
	args := []string{"restart"} //nolint:prealloc
	args = append(args, units...)

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}

// StopUnit instructs systemd to stop the provided unit(s).
func StopUnit(ctx context.Context, units ...string) error {
	args := []string{"stop"} //nolint:prealloc
	args = append(args, units...)

	// If the system is currently shutting down, don't do anything. This addresses
	// a tricky situation where the incus-osd service is stopped by systemd during a
	// poweroff/reboot, and then we hang stopping a socket unit. Something seems to
	// wedge during the shutdown process so that sockets will not stop. This then
	// results in a hard SIGKILL to incus-osd and prevents a clean service shutdown.
	//
	// Attempting to set proper systemd dependencies for incus-osd such as
	// After=sockets.target or Before=incus.socket result in ordering cycles between
	// stop dependencies, and I haven't been able to figure out if it's actually
	// possible to properly express this in a way systemd can handle automatically.
	if IsSystemRunning(ctx) == "stopping" {
		return nil
	}

	_, err := subprocess.RunCommandContext(ctx, "systemctl", args...)
	if err != nil {
		return err
	}

	return nil
}

// IsActive returns a boolean indicating if the specified unit is in an active state.
func IsActive(ctx context.Context, unit string) bool {
	result, err := subprocess.RunCommandContext(ctx, "systemctl", "is-active", unit)
	if err != nil {
		return false
	}

	return result == "active\n"
}

// IsFailed returns a boolean indicating if the specified unit is in a failed state.
func IsFailed(ctx context.Context, unit string) bool {
	result, err := subprocess.RunCommandContext(ctx, "systemctl", "is-failed", unit)
	if err != nil {
		return false
	}

	return result == "failed\n"
}

// IsSystemRunning returns the output of running `systemctl is-system-running` as described at
// https://www.freedesktop.org/software/systemd/man/latest/systemctl.html#is-system-running.
func IsSystemRunning(ctx context.Context) string {
	// Ignore the error, since non-zero exit codes are expected.
	result, _ := subprocess.RunCommandContext(ctx, "systemctl", "is-system-running")

	return strings.TrimSuffix(result, "\n")
}
