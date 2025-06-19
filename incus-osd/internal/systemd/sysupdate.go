package systemd

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

// ErrReleaseNotFound is returned when the os-release file can't be located.
var ErrReleaseNotFound = errors.New("couldn't determine current OS release")

// GetCurrentRelease returns the current NAME and IMAGE_VERSION from the os-release file.
func GetCurrentRelease(_ context.Context) (string, string, error) { //nolint:revive
	// Open the os-release file.
	fd, err := os.Open("/lib/os-release")
	if err != nil {
		return "", "", err
	}

	defer fd.Close()

	name := ""
	version := ""

	// Prepare reader.
	fdScan := bufio.NewScanner(fd)
	for fdScan.Scan() {
		line := fdScan.Text()
		fields := strings.SplitN(line, "=", 2)

		if len(fields) != 2 {
			continue
		}

		switch fields[0] {
		case "NAME":
			name = strings.Trim(fields[1], "\"")
		case "IMAGE_VERSION":
			version = strings.Trim(fields[1], "\"")
		}
	}

	if name != "" && version != "" {
		return name, version, nil
	}

	return "", "", ErrReleaseNotFound
}

// ApplySystemUpdate instructs systemd-sysupdate to apply any pending update and optionally reboot the system.
func ApplySystemUpdate(ctx context.Context, luksPassword string, version string, reboot bool) error {
	// WORKAROUND: Start the boot.mount unit so /boot autofs is active before we create a new mount namespace.
	err := StartUnit(ctx, "boot.mount")
	if err != nil {
		return err
	}

	// Check if the Secure Boot key has changed; if it has apply the necessary updates.
	var newUKIFile string
	var newUsrImageFile string
	updateFiles, err := os.ReadDir(SystemUpdatesPath)
	if err != nil {
		return err
	}

	for _, file := range updateFiles {
		if strings.HasSuffix(file.Name(), "_"+version+".efi") {
			newUKIFile = filepath.Join(SystemUpdatesPath, file.Name())
		} else if strings.Contains(file.Name(), "_"+version+".usr-x86-64.") {
			newUsrImageFile = filepath.Join(SystemUpdatesPath, file.Name())
		}
	}

	secureBootKeyChanged, err := secureboot.UKIHasDifferentSecureBootCertificate(newUKIFile)
	if err != nil {
		return err
	}

	if secureBootKeyChanged {
		err := secureboot.HandleSecureBootKeyChange(ctx, luksPassword, newUKIFile, newUsrImageFile)
		if err != nil {
			return err
		}
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
