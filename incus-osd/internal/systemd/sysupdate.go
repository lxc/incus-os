package systemd

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// ErrReleaseNotFound is returned when the os-release file can't be located.
var ErrReleaseNotFound = errors.New("couldn't determine current OS release")

// GetCurrentRelease returns the current IMAGE_VERSION from the os-release file.
func GetCurrentRelease(_ context.Context) (string, error) {
	// Open the os-release file.
	fd, err := os.Open("/lib/os-release")
	if err != nil {
		return "", err
	}

	defer fd.Close()

	// Prepare reader.
	fdScan := bufio.NewScanner(fd)
	for fdScan.Scan() {
		line := fdScan.Text()
		fields := strings.SplitN(line, "=", 2)

		if len(fields) != 2 {
			continue
		}

		if fields[0] == "IMAGE_VERSION" {
			return strings.Trim(fields[1], "\""), nil
		}
	}

	return "", ErrReleaseNotFound
}

// ApplySystemUpdate instructs systemd-sysupdate to apply any pending update and optionally reboot the system.
func ApplySystemUpdate(ctx context.Context, version string, reboot bool) error {
	// WORKAROUND: Start the boot.mount unit so /boot autofs is active before we create a new mount namespace.
	err := StartUnit(ctx, "boot.mount")
	if err != nil {
		return err
	}

	// WORKAROUND: Needed until systemd-sysupdate can be run with system extensions applied.
	cmd := "mount /dev/mapper/usr /usr && /usr/lib/systemd/systemd-sysupdate update " + version
	if reboot {
		cmd += "&& /usr/lib/systemd/systemd-sysupdate reboot"
	}

	_, err = subprocess.RunCommandContext(ctx, "unshare", "-m", "--", "sh", "-c", cmd)
	if err != nil {
		return err
	}

	// Wait 10s to allow time for the system to reboot.
	time.Sleep(10 * time.Second)

	return nil
}
