package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"
)

// BlockDevices stores specific fields for each device reported by `lsblk`.
type BlockDevices struct {
	KName string `json:"kname"`
	ID    string `json:"id-link"` //nolint:tagliatelle
	Size  int    `json:"size"`
}

// LsblkOutput stores the output of running `lsblk -J ...`.
type LsblkOutput struct {
	BlockDevices []BlockDevices `json:"blockdevices"`
}

type zpoolStatusPartialParse struct {
	Pools map[string]struct {
		Vdevs map[string]struct {
			Vdevs map[string]struct {
				VdevType string `json:"vdev_type"`
				Path     string `json:"path"`
				Vdevs    map[string]struct {
					Path string `json:"path"`
				} `json:"vdevs,omitempty"`
			} `json:"vdevs"`
		} `json:"vdevs"`
		Logs map[string]struct {
			Name string `json:"name"`
		} `json:"logs"`
		L2Cache map[string]struct {
			Name string `json:"name"`
		} `json:"l2cache"`
	} `json:"pools"`
}

// GetUnderlyingDevice figures out and returns the underlying device that Incus OS is running from.
func GetUnderlyingDevice() (string, error) {
	// Determine the device we're running from.
	s := unix.Stat_t{}

	err := unix.Stat("/usr/local/bin/incus-osd", &s)
	if err != nil {
		return "", err
	}

	major := unix.Major(s.Dev)
	minor := unix.Minor(s.Dev)
	rootDev := fmt.Sprintf("%d:%d\n", major, minor)

	// Get a list of all the block devices.
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return "", err
	}

	// Iterate through each of the block devices until we find the one for /usr.
	for _, entry := range entries {
		entryPath := filepath.Join("/sys/class/block", entry.Name())

		dev, err := os.ReadFile(filepath.Join(entryPath, "dev")) //nolint:gosec
		if err != nil {
			continue
		}

		// We've found the mapped device.
		if string(dev) == rootDev {
			// Get the underlying device.
			members, err := os.ReadDir(filepath.Join(entryPath, "slaves"))
			if err != nil {
				return "", err
			}

			// Read the symlink for the underlying device.
			path, err := os.Readlink(filepath.Join(entryPath, "slaves", members[0].Name()))
			if err != nil {
				return "", err
			}

			// We're running from a USB stick.
			if strings.HasPrefix(path, "../../../") {
				// Drop the last element of the path (the partition), then get the base of the resulting path (the actual device).
				parentDir, _ := filepath.Split(path)

				return filepath.Join("/dev/", filepath.Base(parentDir)), nil
			}

			// We're running from a CDROM; need to do one more level of indirection to get the actual device.
			entryPath = filepath.Join("/sys/class/block", filepath.Base(path))

			members, err = os.ReadDir(filepath.Join(entryPath, "slaves"))
			if err != nil {
				return "", err
			}

			return filepath.Join("/dev/", members[0].Name()), nil
		}
	}

	return "", errors.New("unable to determine underlying device")
}

// DeviceToID takes a device path like /dev/sda and determines its "by-id" mapping, for example /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root.
func DeviceToID(ctx context.Context, device string) (string, error) {
	output, err := subprocess.RunCommandContext(ctx, "udevadm", "info", "-q", "symlink", device)
	if err != nil {
		return "", err
	}

	for _, dev := range strings.Split(output, " ") {
		if strings.HasPrefix(dev, "disk/by-id/") {
			dev = strings.TrimSuffix(dev, "\n")

			return "/dev/" + dev, nil
		}
	}

	return "", errors.New("unable to determine device ID for " + device)
}

// IDSymlinkToDevice takes a device symlink like /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root and resolves to symlink to the actual device, for example /dev/sda.
func IDSymlinkToDevice(id string) (string, error) {
	dst, err := os.Readlink(id)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(id), dst), nil
}

// GetZpoolMembers returns all members of a given ZFS pool, organized by normal devices,
// log devices, and cache devices.. Logically it makes more sense for this to be in the
// zfs package, but that would cause an import loop.
func GetZpoolMembers(ctx context.Context, zpoolName string) (map[string][]string, error) {
	output, err := subprocess.RunCommandContext(ctx, "zpool", "status", zpoolName, "-j")
	if err != nil {
		return nil, err
	}

	return getZpoolMembersHelper([]byte(output), zpoolName)
}

func getZpoolMembersHelper(rawJSONContent []byte, zpoolName string) (map[string][]string, error) {
	zpoolJSON := zpoolStatusPartialParse{}

	err := json.Unmarshal(rawJSONContent, &zpoolJSON)
	if err != nil {
		return nil, err
	}

	ret := make(map[string][]string)

	for vdevName, vdev := range zpoolJSON.Pools[zpoolName].Vdevs[zpoolName].Vdevs {
		if vdev.VdevType == "disk" {
			ret["devices"] = append(ret["devices"], "/dev/disk/by-id/"+vdevName)
		} else {
			for memberVdevName := range vdev.Vdevs {
				ret["devices"] = append(ret["devices"], "/dev/disk/by-id/"+memberVdevName)
			}
		}
	}

	for vdevName := range zpoolJSON.Pools[zpoolName].Logs {
		ret["log"] = append(ret["log"], "/dev/disk/by-id/"+vdevName)
	}

	for vdevName := range zpoolJSON.Pools[zpoolName].L2Cache {
		ret["cache"] = append(ret["cache"], "/dev/disk/by-id/"+vdevName)
	}

	return ret, nil
}
