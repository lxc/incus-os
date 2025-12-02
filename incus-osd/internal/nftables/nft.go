package nftables

import (
	"context"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
)

// SetupChains creates the initial system-wide chains.
func SetupChains(ctx context.Context) error {
	// Ensure we have a bridge table.
	_, err := subprocess.RunCommandContext(ctx, "nft", "add", "table", "bridge", "incus-osd")
	if err != nil {
		return err
	}

	// Ensure we have a MAC filtering chain.
	_, err = subprocess.RunCommandContext(ctx, "nft", "add", "chain", "bridge", "incus-osd", "mac-filters", "{ type filter hook output priority 0 ; policy accept ; }")
	if err != nil {
		return err
	}

	return nil
}

// ApplyHwaddrFilters ensures that all interfaces with the StrictHwaddr flag set get a suitable MAC filter in place.
func ApplyHwaddrFilters(ctx context.Context, networkCfg *api.SystemNetworkConfig) error {
	// Make sure we have the expected chains.
	err := SetupChains(ctx)
	if err != nil {
		return err
	}

	// Empty the chain.
	_, err = subprocess.RunCommandContext(ctx, "nft", "flush", "chain", "bridge", "incus-osd", "mac-filters")
	if err != nil {
		return err
	}

	// Apply the filters.
	for _, iface := range networkCfg.Interfaces {
		if !iface.StrictHwaddr {
			continue
		}

		underlyingDevice := "_p" + strings.ToLower(strings.ReplaceAll(iface.Hwaddr, ":", ""))

		_, err = subprocess.RunCommandContext(ctx, "nft", "add", "rule", "bridge", "incus-osd", "mac-filters", "oifname", underlyingDevice, "ether", "saddr", "!=", iface.Hwaddr, "drop")
		if err != nil {
			return err
		}
	}

	return nil
}
