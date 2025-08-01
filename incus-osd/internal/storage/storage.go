package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
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
		State string `json:"state"`
		Vdevs map[string]struct {
			Vdevs map[string]struct {
				VdevType string `json:"vdev_type"`
				State    string `json:"state"`
				Vdevs    map[string]struct {
					State string `json:"state"`
				} `json:"vdevs,omitempty"`
			} `json:"vdevs"`
		} `json:"vdevs"`
		Logs map[string]struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"logs"`
		L2Cache map[string]struct {
			Name  string `json:"name"`
			State string `json:"state"`
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
	if device == "" {
		return "", errors.New("empty string provided for device path")
	}

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
func IDSymlinkToDevice(idSymlink string) (string, error) {
	if idSymlink == "" {
		return "", errors.New("empty string provided for ID symlink")
	}

	dst, err := os.Readlink(idSymlink)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(idSymlink), dst), nil
}

// GetZpoolMembers returns an instantiated SystemStoragePool struct for the specified storage pool.
// Logically it makes more sense for this to be in the zfs package, but that would cause an import loop.
func GetZpoolMembers(ctx context.Context, zpoolName string) (api.SystemStoragePool, error) {
	output, err := subprocess.RunCommandContext(ctx, "zpool", "status", zpoolName, "-j")
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	return getZpoolMembersHelper([]byte(output), zpoolName)
}

func getZpoolMembersHelper(rawJSONContent []byte, zpoolName string) (api.SystemStoragePool, error) {
	zpoolJSON := zpoolStatusPartialParse{}

	err := json.Unmarshal(rawJSONContent, &zpoolJSON)
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolType := ""
	zpoolDevices := make(map[string][]string)

	for vdevName, vdev := range zpoolJSON.Pools[zpoolName].Vdevs[zpoolName].Vdevs {
		if vdev.VdevType == "disk" {
			zpoolType = "zfs-raid0"

			if vdev.State == "ONLINE" {
				zpoolDevices["devices"] = append(zpoolDevices["devices"], "/dev/disk/by-id/"+vdevName)
			} else {
				zpoolDevices["devices_degraded"] = append(zpoolDevices["devices_degraded"], "/dev/disk/by-id/"+vdevName)
			}
		} else {
			switch vdevName {
			case "mirror-0":
				// Only set the zpoolType if it doesn't already have a value; the ordering of vdevs is random.
				if zpoolType == "" {
					zpoolType = "zfs-raid1"
				}
			case "mirror-1", "mirror-2", "mirror-3", "mirror-4", "mirror-5":
				// raid10 gets a bit weird when additional mirror vdevs are added to it...
				zpoolType = "zfs-raid10"
			case "raidz1-0":
				zpoolType = "zfs-raidz1"
			case "raidz2-0":
				zpoolType = "zfs-raidz2"
			case "raidz3-0":
				zpoolType = "zfs-raidz3"
			default:
				return api.SystemStoragePool{}, errors.New("unable to determine pool type for " + zpoolName)
			}

			for memberVdevName, memberVdev := range vdev.Vdevs {
				if memberVdev.State == "ONLINE" {
					zpoolDevices["devices"] = append(zpoolDevices["devices"], "/dev/disk/by-id/"+memberVdevName)
				} else {
					zpoolDevices["devices_degraded"] = append(zpoolDevices["devices_degraded"], "/dev/disk/by-id/"+memberVdevName)
				}
			}
		}
	}

	for vdevName, vdev := range zpoolJSON.Pools[zpoolName].Logs {
		if vdev.State == "ONLINE" {
			zpoolDevices["log"] = append(zpoolDevices["log"], "/dev/disk/by-id/"+vdevName)
		} else {
			zpoolDevices["log_degraded"] = append(zpoolDevices["log_degraded"], "/dev/disk/by-id/"+vdevName)
		}
	}

	for vdevName, vdev := range zpoolJSON.Pools[zpoolName].L2Cache {
		if vdev.State == "ONLINE" {
			zpoolDevices["cache"] = append(zpoolDevices["cache"], "/dev/disk/by-id/"+vdevName)
		} else {
			zpoolDevices["cache_degraded"] = append(zpoolDevices["cache_degraded"], "/dev/disk/by-id/"+vdevName)
		}
	}

	// Sort each list of devices and convert back to nicer short path names.
	zpoolDevices["devices"], err = resolveAndSortDevices(zpoolDevices["devices"])
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolDevices["log"], err = resolveAndSortDevices(zpoolDevices["log"])
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolDevices["cache"], err = resolveAndSortDevices(zpoolDevices["cache"])
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolDevices["devices_degraded"], err = resolveAndSortDevices(zpoolDevices["devices_degraded"])
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolDevices["log_degraded"], err = resolveAndSortDevices(zpoolDevices["log_degraded"])
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolDevices["cache_degraded"], err = resolveAndSortDevices(zpoolDevices["cache_degraded"])
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	return api.SystemStoragePool{
		Name:            zpoolName,
		State:           zpoolJSON.Pools[zpoolName].State,
		Type:            zpoolType,
		Devices:         zpoolDevices["devices"],
		Log:             zpoolDevices["log"],
		Cache:           zpoolDevices["cache"],
		DevicesDegraded: zpoolDevices["devices_degraded"],
		LogDegraded:     zpoolDevices["log_degraded"],
		CacheDegraded:   zpoolDevices["cache_degraded"],
	}, nil
}

func resolveAndSortDevices(devs []string) ([]string, error) {
	for idx, dev := range devs {
		actualDev, err := IDSymlinkToDevice(dev)
		if err != nil {
			return nil, err
		}

		devs[idx] = actualDev
	}

	// Ensure consistent ordering of device names.
	slices.Sort(devs)

	return devs, nil
}
