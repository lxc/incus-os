package systemd

import (
	"context"
	"fmt"
	"os"

	"github.com/lxc/incus-os/incus-osd/api"
)

// SetSyslog sets the system's remote syslog configuration.
func SetSyslog(ctx context.Context, syslog api.SystemLoggingSyslog) error {
	// Handle disabling logging.
	if syslog.Address == "" {
		err := os.Remove("/etc/systemd/netlogd.conf")
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		return StopUnit(ctx, "systemd-netlogd")
	}

	// Set defaults.
	if syslog.Protocol == "" {
		syslog.Protocol = "udp"
	}

	if syslog.LogFormat == "" {
		syslog.LogFormat = "rfc5424"
	}

	// Write the configuration.
	w, err := os.Create("/etc/systemd/netlogd.conf")
	if err != nil {
		return err
	}

	defer func() { _ = w.Close() }()

	_, err = fmt.Fprintf(w, `[Network]
Address=%s
Protocol=%s
LogFormat=%s
`, syslog.Address, syslog.Protocol, syslog.LogFormat)
	if err != nil {
		return err
	}

	// Start the daemon.
	return RestartUnit(ctx, "systemd-netlogd")
}
