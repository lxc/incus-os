package services

import (
	"context"
	"log/slog"
	"slices"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// ApplySeed configures services from the provided seed data.
func ApplySeed(ctx context.Context, s *state.State, seedConfig *apiseed.Services) {
	configs := map[string]any{}

	if seedConfig.ISCSI != nil {
		configs["iscsi"] = &api.ServiceISCSI{Config: *seedConfig.ISCSI}
	}

	if seedConfig.LVM != nil {
		configs["lvm"] = &api.ServiceLVM{Config: *seedConfig.LVM}
	}

	if seedConfig.Multipath != nil {
		configs["multipath"] = &api.ServiceMultipath{Config: *seedConfig.Multipath}
	}

	if seedConfig.Netbird != nil {
		configs["netbird"] = &api.ServiceNetbird{Config: *seedConfig.Netbird}
	}

	if seedConfig.NVME != nil {
		configs["nvme"] = &api.ServiceNVME{Config: *seedConfig.NVME}
	}

	if seedConfig.OVN != nil {
		configs["ovn"] = &api.ServiceOVN{Config: *seedConfig.OVN}
	}

	if seedConfig.Tailscale != nil {
		configs["tailscale"] = &api.ServiceTailscale{Config: *seedConfig.Tailscale}
	}

	if seedConfig.USBIP != nil {
		configs["usbip"] = &api.ServiceUSBIP{Config: *seedConfig.USBIP}
	}

	// Apply the configurations in service startup order.
	supported := Supported(s)

	for _, name := range supported {
		req, ok := configs[name]
		if !ok {
			continue
		}

		srv, err := Load(ctx, s, name)
		if err != nil {
			slog.ErrorContext(ctx, "Failed loading seeded service", "name", name, "err", err)

			continue
		}

		slog.InfoContext(ctx, "Applying seed configuration to service", "name", name)

		err = srv.Update(ctx, req)
		if err != nil {
			slog.ErrorContext(ctx, "Failed configuring seeded service", "name", name, "err", err)
		}
	}

	// Warn about seeded services that aren't currently supported.
	for name := range configs {
		if !slices.Contains(supported, name) {
			slog.WarnContext(ctx, "Skipping seed configuration for unsupported service", "name", name)
		}
	}
}
