package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	RM    bool   `json:"rm"`
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
				VdevType   string `json:"vdev_type"`
				State      string `json:"state"`
				AllocSpace int    `json:"alloc_space"`
				TotalSpace int    `json:"total_space"`
				DefSpace   int    `json:"def_space"`
				Vdevs      map[string]struct {
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

type zfsGetPartialParse struct {
	Datasets map[string]struct {
		Properties map[string]struct {
			Value string `json:"value"`
		} `json:"properties"`
	} `json:"datasets"`
}

type smartOutput struct {
	Device struct {
		Type string `json:"type"`
	} `json:"device"`
	ModelFamily  string `json:"model_family"`
	ModelName    string `json:"model_name"`
	SCSIVendor   string `json:"scsi_vendor"`
	SCSIProduct  string `json:"scsi_product"`
	SerialNumber string `json:"serial_number"`
	WWN          struct {
		NAA int `json:"naa"`
		OUI int `json:"oui"`
		ID  int `json:"id"`
	} `json:"wwn"`
	SMARTSupport struct {
		Available bool `json:"available"`
		Enabled   bool `json:"enabled"`
	} `json:"smart_support"`
	SMARTStatus struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
}

// GetUnderlyingDevice figures out and returns the underlying device that IncusOS is running from.
func GetUnderlyingDevice() (string, error) {
	// We need to find a file that's on a device mapper device and not on overlayfs.
	var rootDev string

	for _, filePath := range []string{"/usr/local/bin/incus-osd", "/etc/passwd"} {
		// Determine the device we're running from.
		s := unix.Stat_t{}

		// Check the backing device for the file.
		err := unix.Stat(filePath, &s)
		if err != nil {
			continue
		}

		major := unix.Major(s.Dev)
		minor := unix.Minor(s.Dev)

		// We want to see device mapper.
		if major != 252 {
			continue
		}

		rootDev = fmt.Sprintf("%d:%d\n", major, minor)
	}

	if rootDev == "" {
		return "", errors.New("couldn't find a file on device mapper")
	}

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

// GetFreeSpaceInGiB returns the amount of free space in GiB for the underlying filesystem of the given path.
func GetFreeSpaceInGiB(path string) (float64, error) {
	var s unix.Statfs_t

	err := unix.Statfs(path, &s)
	if err != nil {
		return 0.0, err
	}

	return float64(s.Bsize*int64(s.Bfree)) / 1024.0 / 1024.0 / 1024.0, nil //nolint:gosec
}

// DeviceToID takes a device path like /dev/sda and determines its "by-id" mapping, for example /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root.
func DeviceToID(ctx context.Context, device string) (string, error) {
	if device == "" {
		return "", errors.New("empty string provided for device path")
	}

	// If the device is already mapped, no need to do anything else.
	if strings.HasPrefix(device, "/dev/disk/by-id/") {
		return device, nil
	}

	output, err := subprocess.RunCommandContext(ctx, "udevadm", "info", "-q", "symlink", device)
	if err != nil {
		return "", err
	}

	// Sort the list of returned symlinks so we consistently return the same symlink.
	candidates := strings.Split(output, " ")
	slices.Sort(candidates)

	for _, dev := range candidates {
		if strings.HasPrefix(dev, "disk/by-id/") {
			dev = strings.TrimSuffix(dev, "\n")

			return "/dev/" + dev, nil
		}
	}

	return "", errors.New("unable to determine device ID for " + device)
}

// PoolExists checks if a given ZFS pool exists.
func PoolExists(ctx context.Context, zpoolName string) bool {
	_, err := subprocess.RunCommandContext(ctx, "zpool", "status", zpoolName)

	return err == nil
}

// DatasetExists checks if a given ZFS dataset exists.
func DatasetExists(ctx context.Context, datasetName string) bool {
	_, err := subprocess.RunCommandContext(ctx, "zfs", "list", datasetName)

	return err == nil
}

// GetZpoolMembers returns an instantiated SystemStoragePool struct for the specified storage pool.
// Logically it makes more sense for this to be in the zfs package, but that would cause an import loop.
func GetZpoolMembers(ctx context.Context, zpoolName string) (api.SystemStoragePool, error) {
	output, err := subprocess.RunCommandContext(ctx, "zpool", "status", zpoolName, "-jp", "--json-int")
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	return getZpoolMembersHelper(ctx, []byte(output), zpoolName)
}

func getZpoolMembersHelper(ctx context.Context, rawJSONContent []byte, zpoolName string) (api.SystemStoragePool, error) {
	zpoolJSON := zpoolStatusPartialParse{}

	err := json.Unmarshal(rawJSONContent, &zpoolJSON)
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	// Get the encryption key status.
	zfsGetOutput, err := subprocess.RunCommandContext(ctx, "zfs", "get", "keystatus", zpoolName, "-j")
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zfsProperties := zfsGetPartialParse{}

	err = json.Unmarshal([]byte(zfsGetOutput), &zfsProperties)
	if err != nil {
		return api.SystemStoragePool{}, err
	}

	zpoolType := ""
	zpoolAllocSpace := 0
	zpoolTotalSpace := 0
	zpoolDefSpace := 0
	zpoolDevices := make(map[string][]string)

	for vdevName, vdev := range zpoolJSON.Pools[zpoolName].Vdevs[zpoolName].Vdevs {
		if vdev.VdevType == "disk" {
			zpoolType = "zfs-raid0"

			// For installs before the storage API was implemented, the "local" ZFS pool was created using
			// partition labels, rather than partition/disk IDs. If vdevName is "local-data", then tweak
			// the parentDir.
			parentDir := "/dev/disk/by-id/"
			if vdevName == "local-data" {
				parentDir = "/dev/disk/by-partlabel/"
			}

			if vdev.State == "ONLINE" {
				zpoolDevices["devices"] = append(zpoolDevices["devices"], parentDir+vdevName)
			} else {
				zpoolDevices["devices_degraded"] = append(zpoolDevices["devices_degraded"], parentDir+vdevName)
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

		zpoolAllocSpace += vdev.AllocSpace
		zpoolTotalSpace += vdev.TotalSpace
		zpoolDefSpace += vdev.DefSpace
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

	// Sort each list of devices to ensure consistent ordering of device names.
	slices.Sort(zpoolDevices["devices"])
	slices.Sort(zpoolDevices["log"])
	slices.Sort(zpoolDevices["cache"])
	slices.Sort(zpoolDevices["devices_degraded"])
	slices.Sort(zpoolDevices["log_degraded"])
	slices.Sort(zpoolDevices["cache_degraded"])

	return api.SystemStoragePool{
		Name:                      zpoolName,
		State:                     zpoolJSON.Pools[zpoolName].State,
		EncryptionKeyStatus:       zfsProperties.Datasets[zpoolName].Properties["keystatus"].Value,
		Type:                      zpoolType,
		Devices:                   zpoolDevices["devices"],
		Log:                       zpoolDevices["log"],
		Cache:                     zpoolDevices["cache"],
		DevicesDegraded:           zpoolDevices["devices_degraded"],
		LogDegraded:               zpoolDevices["log_degraded"],
		CacheDegraded:             zpoolDevices["cache_degraded"],
		RawPoolSizeInBytes:        zpoolTotalSpace,
		UsablePoolSizeInBytes:     zpoolDefSpace,
		PoolAllocatedSpaceInBytes: zpoolAllocSpace,
	}, nil
}

// GetStorageInfo returns current SMART data for each drive and the status of each local zpool.
func GetStorageInfo(ctx context.Context) (api.SystemStorage, error) {
	ret := api.SystemStorage{}

	type zpoolStatusRaw struct {
		Pools map[string]json.RawMessage `json:"pools"`
	}

	// Get the status of the zpool(s).
	zpoolOutput, err := subprocess.RunCommandContext(ctx, "zpool", "status", "-jp", "--json-int")
	if err != nil {
		return ret, err
	}

	zpools := zpoolStatusRaw{}

	err = json.Unmarshal([]byte(zpoolOutput), &zpools)
	if err != nil {
		return ret, err
	}

	// Populate the Config.State struct.
	for zpoolName := range zpools.Pools {
		poolConfig, err := getZpoolMembersHelper(ctx, []byte(zpoolOutput), zpoolName)
		if err != nil {
			return ret, err
		}

		ret.State.Pools = append(ret.State.Pools, poolConfig)
	}

	// Get a list of all local drives.
	// Note that while we can get the VENDOR field from lsblk, it seems to return generic values like "ATA" which isn't useful.
	// Exclude devices with major numbers 1 (RAM disk), 2 (floppy disks), 7 (loopback), 230 (zvols)
	output, err := subprocess.RunCommandContext(ctx, "lsblk", "-JMpdb", "-e", "1,2,7,230", "-o", "KNAME,SIZE,RM")
	if err != nil {
		return ret, err
	}

	drives := LsblkOutput{}

	err = json.Unmarshal([]byte(output), &drives)
	if err != nil {
		return ret, err
	}

	bootDevice, err := GetUnderlyingDevice()
	if err != nil {
		return ret, err
	}

	// Get SMART data and populate struct for each drive.
	for _, drive := range drives.BlockDevices {
		// Ignore error here, since smartctl returns non-zero if the device doesn't support SMART, such as a QEMU virtual drive.
		output, _ := subprocess.RunCommandContext(ctx, "smartctl", "-aj", drive.KName)

		smart := smartOutput{}

		err = json.Unmarshal([]byte(output), &smart)
		if err != nil {
			return ret, err
		}

		// Determine if this is a remote device (NVMEoTCP, FC, etc).
		isRemote, err := IsRemoteDevice(drive.KName)
		if err != nil {
			return ret, err
		}

		// If model_family or model_name are empty, try to populate values by looking at SCSI fields.
		modelFamily := smart.ModelFamily
		if modelFamily == "" {
			modelFamily = smart.SCSIVendor
		}

		modelName := smart.ModelName
		if modelName == "" {
			modelName = smart.SCSIProduct
		}

		// Build a hex WWN string.
		wwnString := ""
		if smart.WWN.NAA != 0 && smart.WWN.OUI != 0 && smart.WWN.ID != 0 {
			wwnString = fmt.Sprintf("0x%x", (smart.WWN.NAA<<60)+(smart.WWN.OUI<<36)+smart.WWN.ID)
		}

		// Resolve the device name to a more stable by-id symlink.
		deviceID, err := DeviceToID(ctx, drive.KName)
		if err != nil {
			return ret, err
		}

		// If we have a WWN, prefer that over other potential by-id symlinks.
		if wwnString != "" {
			deviceID = "/dev/disk/by-id/wwn-" + wwnString
		}

		// Check if the drive belongs to a zpool.
		driveZpool := ""

		for zpoolName := range zpools.Pools {
			poolConfig, err := getZpoolMembersHelper(ctx, []byte(zpoolOutput), zpoolName)
			if err != nil {
				return ret, err
			}

			if isMemberDrive(poolConfig.Devices, deviceID) || isMemberDrive(poolConfig.Log, deviceID) || isMemberDrive(poolConfig.Cache, deviceID) ||
				isMemberDrive(poolConfig.DevicesDegraded, deviceID) || isMemberDrive(poolConfig.LogDegraded, deviceID) || isMemberDrive(poolConfig.CacheDegraded, deviceID) {
				driveZpool = zpoolName

				break
			}
		}

		// Populate SMART info if available.
		smartStatus := new(api.SystemStorageDriveSMART)
		if smart.SMARTSupport.Available {
			smartStatus.Enabled = smart.SMARTSupport.Enabled
			smartStatus.Passed = smart.SMARTStatus.Passed
		} else {
			smartStatus = nil
		}

		ret.State.Drives = append(ret.State.Drives, api.SystemStorageDrive{
			ID:              deviceID,
			ModelFamily:     modelFamily,
			ModelName:       modelName,
			SerialNumber:    smart.SerialNumber,
			Bus:             smart.Device.Type,
			CapacityInBytes: drive.Size,
			Boot:            drive.KName == bootDevice,
			Removable:       drive.RM,
			Remote:          isRemote,
			WWN:             wwnString,
			SMART:           smartStatus,
			MemberPool:      driveZpool,
		})
	}

	// Sort the list of returned drives by the device's ID. This ensures a
	// consistent ordering, which is useful for some tests.
	slices.SortFunc(ret.State.Drives, func(a, b api.SystemStorageDrive) int {
		return strings.Compare(a.ID, b.ID)
	})

	return ret, nil
}

// Helper function that checks if a given drive is a member of the provided list of drives.
// Because multiple symlinks can point to the same underlying drive, we resolve the symlinks
// before checking if the drive matches one of the drives in the provided list.
func isMemberDrive(list []string, drive string) bool {
	// Resolve drive's symlink.
	deviceDst, err := os.Readlink(drive)
	if err != nil {
		return false
	}

	// Update the path we're checking for.
	drive = filepath.Join(filepath.Dir(drive), deviceDst)

	// Iterate through each of the drives in the list and see if it's the same as the one we're searching for.
	for _, dev := range list {
		devDst, err := os.Readlink(dev)
		if err != nil {
			continue
		}

		if filepath.Join(filepath.Dir(dev), devDst) == drive {
			return true
		}
	}

	return false
}

// IsRemoteDevice determines if a given device is remote (NVMEoTCP, FC, etc).
func IsRemoteDevice(deviceName string) (bool, error) {
	// We might be given a symlink, such as /dev/disk/by-id/....; if so, first resolve it to the actual device.
	symlink, err := os.Readlink(deviceName)
	if err == nil {
		deviceName = filepath.Join(deviceName, symlink)
	}

	device := filepath.Base(deviceName)

	// Check if we have been given a partition; if so, get the device it belongs to.
	_, err = os.Stat("/sys/class/block/" + device + "/partition")
	if err == nil {
		symlink, err := os.Readlink("/sys/class/block/" + device)
		if err == nil {
			device = filepath.Base(filepath.Join(symlink, ".."))
		}
	}

	// SATA or FC.
	if strings.HasPrefix(device, "sd") {
		link, err := os.Readlink("/sys/class/block/" + device)
		if err != nil {
			return false, err
		}

		// If the symlink contains "/rport-", it's a remote FC device, otherwise it's local.
		return strings.Contains(link, "/rport-"), nil
	}

	// NVME.
	if strings.HasPrefix(device, "nvme") {
		re := regexp.MustCompile(`n\d+$`)
		nvmeDevice := re.ReplaceAllString(device, "")

		// Attempt to read the specific symlink for the underlying device. This may not exist
		// on some systems, and if so we'll fallback to directly considering the device's symlink.
		link, err := os.Readlink("/sys/class/block/" + device + "/device/" + nvmeDevice)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return false, err
			}

			// Try the actual device symlink.
			link, err = os.Readlink("/sys/class/block/" + device)
			if err != nil {
				return false, err
			}
		}

		// If the symlink contains "/pci", it's local, otherwise it's remote.
		return !strings.Contains(link, "/pci"), nil
	}

	// QEMU drive.
	if strings.HasPrefix(device, "vd") {
		return false, nil
	}

	// Default to saying the device is local.
	return false, nil
}

// WipeDrive will wipe all data on the given drive, unless it is the boot device,
// a remote device, or currently a member of a storage pool.
func WipeDrive(ctx context.Context, drive string) error {
	// Get a list of all drives.
	drives, err := GetStorageInfo(ctx)
	if err != nil {
		return err
	}

	for _, d := range drives.State.Drives {
		if d.ID == drive {
			if d.Boot {
				return errors.New("cannot wipe boot drive")
			} else if d.MemberPool != "" {
				return errors.New("cannot wipe drive belonging to pool '" + d.MemberPool + "'")
			}

			// Wipe the drive.
			return ClearBlock(drive, 0)
		}
	}

	return errors.New("drive '" + drive + "' doesn't exist")
}

// SetEncryptionKey will save a local copy of the provided encryption key for the associated pool.
// If a local copy of the encryption key already exists, return an error and refuse to continue.
func SetEncryptionKey(ctx context.Context, pool string, key string) error {
	if !PoolExists(ctx, pool) {
		return errors.New("pool '" + pool + "' doesn't exist")
	}

	keyfilePath := "/var/lib/incus-os/zpool." + pool + ".key"

	// Check if the key already exists.
	_, err := os.Stat(keyfilePath)
	if err == nil {
		return errors.New("encryption key for '" + pool + "' already exists")
	}

	// Decode into raw bytes.
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}

	if len(rawKey) != 32 {
		return fmt.Errorf("expected a 32 byte raw encryption key, got %d bytes", len(rawKey))
	}

	// Write the key file.
	// #nosec G304
	err = os.WriteFile(keyfilePath, rawKey, 0o0600)
	if err != nil {
		return err
	}

	// Load the pool's encryption key.
	_, err = subprocess.RunCommandContext(ctx, "zfs", "load-key", pool)
	if err != nil {
		// Cleanup the invalid key.
		_ = os.Remove(keyfilePath)
	}

	return err
}
