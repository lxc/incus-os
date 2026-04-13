package systemd

import (
	"context"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// FlushDNSCache instructs the system to flush the DNS cache. This is needed after network changes to ensure that any cached DNS entries that may have become stale are cleared.
func FlushDNSCache(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "resolvectl", "flush-caches")
	if err != nil {
		return err
	}

	return nil
}
