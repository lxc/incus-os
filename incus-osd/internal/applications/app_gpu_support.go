package applications

import (
	"context"
	"log/slog"
	"os"

	"github.com/lxc/incus/v6/shared/subprocess"
)

type gpuSupport struct {
	common
}

func (*gpuSupport) Name() string {
	return "gpu-support"
}

func (*gpuSupport) Start(ctx context.Context, _ string) error {
	// Reload the modules if loaded.
	for _, module := range []string{"amdgpu", "i915", "nouveau"} {
		// Check if loaded.
		_, err := os.Stat("/sys/module/" + module)
		if err != nil {
			continue
		}

		// Unload the module.
		_, err = subprocess.RunCommandContext(ctx, "/sbin/rmmod", module)
		if err != nil {
			slog.Warn("Failed to unload kernel module", "module", module)

			continue
		}

		// Load the module back.
		_, err = subprocess.RunCommandContext(ctx, "/sbin/modprobe", module)
		if err != nil {
			slog.Warn("Failed to load kernel module", "module", module)

			continue
		}
	}

	return nil
}
