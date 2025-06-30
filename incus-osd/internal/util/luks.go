package util //nolint:revive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetLUKSVolumePartitions returns the underlying partitions that hold the root and swap LUKS volumes.
// We can't just rely on /dev/disk/by-partlabel/root-x86-64, because as soon as an overlay is applied
// that symlink is repointed to the newly mapped loop device.
func GetLUKSVolumePartitions() ([]string, error) {
	// /dev/disk/by-partlabel/swap should always point to the correct underlying device.
	linkDest, err := os.Readlink("/dev/disk/by-partlabel/swap")
	if err != nil {
		return nil, err
	}

	absSwapDev := filepath.Join("/dev/disk/by-partlabel", linkDest)

	absRootDev, found := strings.CutSuffix(absSwapDev, "9")
	if !found {
		return nil, fmt.Errorf("unexpected swap device: '%s'", absSwapDev)
	}

	absRootDev += "10"

	return []string{absRootDev, absSwapDev}, nil
}
