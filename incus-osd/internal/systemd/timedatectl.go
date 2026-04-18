package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
)

// SetTimezone updates the system's timezone.
func SetTimezone(ctx context.Context, timeCfg *api.SystemNetworkTime) error {
	if timeCfg == nil || timeCfg.Timezone == "" {
		return nil
	}

	_, err := subprocess.RunCommandContext(ctx, "timedatectl", "set-timezone", timeCfg.Timezone)

	return err
}
