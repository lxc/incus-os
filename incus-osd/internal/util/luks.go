package util

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// GetLUKSVolumePartitions returns the underlying partitions that hold the root and swap LUKS volumes.
// We can't just rely on /dev/disk/by-partlabel/root-ARCH, because as soon as an overlay is applied
// that symlink is repointed to the newly mapped loop device.
func GetLUKSVolumePartitions(ctx context.Context) (map[string]string, error) {
	// /dev/disk/by-partlabel/swap should always point to the correct underlying device.
	linkDest, err := os.Readlink("/dev/disk/by-partlabel/swap")
	if err != nil {
		return nil, err
	}

	absSwapDev := filepath.Join("/dev/disk/by-partlabel", linkDest)

	// When running on a boot device that is multipath-backed, each partition
	// will actually be an individually mapped device. In this case, look up the
	// nice symlink and use that in the returned values.
	if strings.HasPrefix(absSwapDev, "/dev/dm-") {
		absSwapDev, err = ResolveMapperSymlink(ctx, absSwapDev)
		if err != nil {
			return nil, err
		}
	}

	absRootDev, found := strings.CutSuffix(absSwapDev, "9")
	if !found {
		return nil, fmt.Errorf("unexpected swap device: '%s'", absSwapDev)
	}

	absRootDev += "10"

	return map[string]string{
		"root": absRootDev,
		"swap": absSwapDev,
	}, nil
}

// ResolveMapperSymlink uses the "dmsetup info" command to get a "nice" symlink
// for the mapped device.
func ResolveMapperSymlink(ctx context.Context, mappedDev string) (string, error) {
	output, err := subprocess.RunCommandContext(ctx, "dmsetup", "info", mappedDev)
	if err != nil {
		return "", err
	}

	nameRegex := regexp.MustCompile(`Name:\s+(.+)`)

	return "/dev/mapper/" + nameRegex.FindStringSubmatch(output)[1], nil
}
