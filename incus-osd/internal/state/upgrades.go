package state

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"
)

var keyFileRegex = regexp.MustCompile(`^(luks|zpool|recovery)\..+\.key$`)

// UpgradeFuncs is a list of functions to apply in order to upgrade the version of a given state.
// Each function consumes a list of strings, each representing one line of input, and returns a
// list of strings representing the upgraded state.
type UpgradeFuncs []func([]string) ([]string, error)

// upgrades is a list of upgrade functions to process old states. Very old upgrade functions
// are removed to minimize the amount of cruft accumulation over time.
var upgrades = UpgradeFuncs{
	// V1: struct System.Encryption renamed to System.Security, along with renaming of a couple of fields.
	nil,
	// V2: struct Network.Proxy expended to support switch to using kpx for proxying.
	nil,
	// V3: Applications have fields moved under State struct.
	nil,
	// V4: Set default value for channel list.
	nil,
	// V5: Switch CheckFrequency to be a string.
	nil,
	// V6: Rename SystemNetworkNTP struct to SystemNetworkTime.
	nil,
	// V7: Set default value for ScrubSchedule.
	nil,
	// V8: Rewrite application state entries.
	func(lines []string) ([]string, error) {
		appNames := map[string]string{
			"debug":             "Debug",
			"gpu-support":       "GPUSupport",
			"incus":             "Incus",
			"incus-ceph":        "IncusCeph",
			"incus-linstor":     "IncusLinstor",
			"migration-manager": "MigrationManager",
			"operations-center": "OperationsCenter",
		}

		for i, line := range lines {
			if strings.HasPrefix(line, "Applications[") {
				for oldName, newName := range appNames {
					lines[i] = strings.Replace(lines[i], "["+oldName+"]", "."+newName, 1)
				}
			}
		}

		return lines, nil
	},
	// V9: Set default value for TrimSchedule.
	func(lines []string) ([]string, error) {
		lines = append(lines, "System.Storage.Config.TrimSchedule: 0 4 * * 6")

		return lines, nil
	},
	// V10: Move encryption keys to /var/lib/incus-os/keys/.
	func(lines []string) ([]string, error) {
		files, err := os.ReadDir("/var/lib/incus-os/")
		if err != nil {
			// Nothing to move on systems without a state directory.
			if errors.Is(err, os.ErrNotExist) {
				return lines, nil
			}

			return nil, err
		}

		err = os.MkdirAll("/var/lib/incus-os/keys/", 0o700)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if file.IsDir() || !keyFileRegex.MatchString(file.Name()) {
				continue
			}

			err = os.Rename(filepath.Join("/var/lib/incus-os/", file.Name()), filepath.Join("/var/lib/incus-os/keys/", file.Name()))
			if err != nil {
				return nil, err
			}

			// Point ZFS pools at the new key file location.
			pool, ok := strings.CutPrefix(file.Name(), "zpool.")
			if ok {
				err = updateZpoolKeyLocation(strings.TrimSuffix(pool, ".key"))
				if err != nil {
					return nil, err
				}
			}
		}

		return lines, nil
	},
}

// updateZpoolKeyLocation sets the pool's keylocation to its key file in /var/lib/incus-os/keys/.
func updateZpoolKeyLocation(pool string) error {
	ctx := context.Background()

	// Import the pool unless it already is.
	_, err := subprocess.RunCommandContext(ctx, "zpool", "import", pool)
	if err != nil && !strings.Contains(err.Error(), "cannot import '"+pool+"': a pool with that name already exists") {
		slog.Warn("Unable to import storage pool to update its key location", "pool", pool, "err", err)

		return nil
	}

	_, err = subprocess.RunCommandContext(ctx, "zfs", "set", "keylocation=file:///var/lib/incus-os/keys/zpool."+pool+".key", pool)

	return err
}
