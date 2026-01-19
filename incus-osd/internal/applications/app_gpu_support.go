package applications

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

type gpuSupport struct {
	common
}

func (*gpuSupport) Name() string {
	return "gpu-support"
}

func (*gpuSupport) IsRunning(_ context.Context) bool {
	return true
}

func (*gpuSupport) Start(ctx context.Context, _ string) error {
	// Reload the modules if loaded.
	for _, module := range []string{"amdgpu", "i915", "nouveau"} {
		// Check if loaded.
		_, err := os.Stat("/sys/module/" + module)
		if err != nil {
			continue
		}

		// Unbind the devices.
		entries, err := os.ReadDir("/sys/bus/pci/drivers/" + module)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("Failed to check kernel module", "module", module)

			continue
		}

		for _, entry := range entries {
			address := filepath.Base(entry.Name())

			if !strings.Contains(address, ":") || !strings.Contains(address, ".") {
				continue
			}

			err := os.WriteFile("/sys/bus/pci/drivers/"+module+"/unbind", []byte(address), 0o600)
			if err != nil {
				slog.Warn("Failed to unbind device", "module", module, "address", address)

				continue
			}
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
