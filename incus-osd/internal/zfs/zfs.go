package zfs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/revert"
	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

const (
	// PoolScrubJob represents the job to scrub all storage pools.
	PoolScrubJob scheduling.JobName = "pool_scrub"
)

var supportedPoolTypes = []string{"zfs-raid0", "zfs-raid1", "zfs-raid10", "zfs-raidz1", "zfs-raidz2", "zfs-raidz3"}

// LoadPools will import all managed ZFS pools on the local system and attempt to load
// their corresponding encryption keys. If the "local" pool doesn't exist, it will also
// be created as an encrypted ZFS pool in the partition labeled "local-data".
func LoadPools(ctx context.Context, s *state.State) error {
	// Get pools for which we have an encryption key stored locally.
	pools, err := getPoolsWithKnownKeys()
	if err != nil {
		return err
	}

	// For each pool we manage, import the pool and load its encryption key.
	for _, pool := range pools {
		_, err = subprocess.RunCommandContext(ctx, "zpool", "import", pool)
		if err != nil {
			// If the pool doesn't exist, log a warning and allow startup to continue.
			if strings.Contains(err.Error(), "cannot import '"+pool+"': no such pool available") {
				slog.WarnContext(ctx, "Unable to import storage pool '"+pool+"', its contents will be unavailable")

				continue
			}

			// If the pool is already imported, don't return an error.
			if !strings.Contains(err.Error(), "cannot import '"+pool+"': a pool with that name already exists") {
				return err
			}
		}

		_, err = subprocess.RunCommandContext(ctx, "zfs", "load-key", pool)
		if err != nil {
			// If the pool's encryption key has already been loaded, don't return an error.
			if !strings.Contains(err.Error(), "Key load error: Key already loaded for") {
				return err
			}
		}
	}

	// If the "local" pool isn't automatically imported, this is a first boot and we either
	// need to create a fresh "local" pool or attempt to recover an existing pool.
	if !storage.PoolExists(ctx, "local") {
		_, err := subprocess.RunCommandContext(ctx, "zpool", "import", "local")
		if err != nil {
			// Failed to import the pool, so create a fresh one.
			zpool := api.SystemStoragePool{
				Name:    "local",
				Type:    "zfs-raid0",
				Devices: []string{"/dev/disk/by-partlabel/local-data"},
			}

			err := CreateZpool(ctx, zpool, s)
			if err != nil {
				return err
			}
		} else {
			// We were able to import the existing "local" pool.
			err := recoverLocalPool(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func recoverLocalPool(ctx context.Context) error {
	poolConfig, err := storage.GetZpoolMembers(ctx, "local")
	if err != nil {
		return err
	}

	// Check if the "local" pool is degraded and consists of two devices. If so, attempt to recover the pool
	// if we're missing the partition on the main system drive (ie, the main disk died and IncusOS was reinstalled),
	// otherwise display a warning to the user about the degraded state.
	if poolConfig.State == "DEGRADED" && len(poolConfig.Devices) == 1 && len(poolConfig.DevicesDegraded) == 1 { //nolint:nestif
		actualrootDev, err := storage.DeviceToID(ctx, "/dev/disk/by-partlabel/local-data", false)
		if err != nil {
			return err
		}

		if poolConfig.Devices[0] == actualrootDev {
			slog.WarnContext(ctx, "Storage pool 'local' is degraded; the second non-system drive appears to be missing")
		} else {
			slog.InfoContext(ctx, "Attempting to recover storage pool 'local' using existing non-system drive")

			_, err := subprocess.RunCommandContext(ctx, "zpool", "replace", "local", filepath.Base(poolConfig.DevicesDegraded[0]), actualrootDev)
			if err != nil {
				return err
			}

			// Need to wait for the resilver to finish before exporting the pool. Otherwise, the pool
			// will still be in a degraded state when it's imported again.
			for {
				poolConfig, err = storage.GetZpoolMembers(ctx, "local")
				if err != nil {
					return err
				}

				if poolConfig.State == "ONLINE" {
					break
				}

				time.Sleep(1 * time.Second)
			}
		}
	} else {
		slog.WarnContext(ctx, "Storage pool 'local' is missing its encryption key")
	}

	// Export the "local" pool. This keeps the logic for allowing the user to set the encryption recovery key
	// via the import-pool API simple.
	_, err = subprocess.RunCommandContext(ctx, "zpool", "export", "local")

	return err
}

// Helper function to return a list of ZFS pools that have a corresponding known encryption key saved locally.
func getPoolsWithKnownKeys() ([]string, error) {
	files, err := os.ReadDir("/var/lib/incus-os/")
	if err != nil {
		return nil, err
	}

	ret := []string{}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "zpool.") && strings.HasSuffix(file.Name(), ".key") {
			ret = append(ret, strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".key"), "zpool."))
		}
	}

	return ret, nil
}

// CreateZpool creates a new zpool.
func CreateZpool(ctx context.Context, zpool api.SystemStoragePool, s *state.State) error { //nolint:revive
	keyfilePath := "/var/lib/incus-os/zpool." + zpool.Name + ".key"

	// Verify a zpool name was provided.
	if zpool.Name == "" {
		return errors.New("a name for the zpool must be provided")
	}

	// Check if the zpool already exists.
	if storage.PoolExists(ctx, zpool.Name) {
		return errors.New("zpool '" + zpool.Name + "' already exists")
	}

	// Check if an encryption key already exists.
	_, err := os.Stat(keyfilePath)
	if err == nil {
		return errors.New("encryption key for '" + zpool.Name + "' already exists")
	}

	// Verify we are given a supported type.
	if !slices.Contains(supportedPoolTypes, zpool.Type) {
		return errors.New("unsupported pool type " + zpool.Type)
	}

	// Verify at least one device was specified.
	if len(zpool.Devices) == 0 {
		return errors.New("at least one device must be specified")
	}

	// Verify at least two devices were specified if asked to create a mirror.
	if zpool.Type == "zfs-raid1" && len(zpool.Devices) < 2 {
		return errors.New("at least two devices must be specified when creating a raid1 pool")
	}

	// If asked to create a raid10 pool, ensure an even number of devices greater than or equal to four was specified.
	if zpool.Type == "zfs-raid10" {
		if len(zpool.Devices) < 4 {
			return errors.New("at least four devices must be specified when creating a raid10 pool")
		} else if len(zpool.Devices)%2 != 0 {
			return errors.New("an even number of devices must be specified when creating a raid10 pool")
		}
	}

	// Basic special vdev checks.
	if zpool.Special != nil {
		// Verify we are given a supported type.
		if !slices.Contains(supportedPoolTypes, zpool.Special.Type) {
			return errors.New("unsupported special device type " + zpool.Special.Type)
		}

		// Verify at least two devices were specified if asked to create a mirror svdev.
		if zpool.Special.Type == "zfs-raid1" && len(zpool.Special.Devices) < 2 {
			return errors.New("at least two devices must be specified when creating a raid1 special vdev")
		}

		// If asked to create a raid10 svdev, ensure an even number of devices greater than or equal to four was specified.
		if zpool.Special.Type == "zfs-raid10" {
			if len(zpool.Special.Devices) < 4 {
				return errors.New("at least four devices must be specified when creating a raid10 special vdev")
			} else if len(zpool.Special.Devices)%2 != 0 {
				return errors.New("an even number of devices must be specified when creating a raid10 special vdev")
			}
		}
	}

	// Simple check if the root drive is in the list of devices for this new zpool.
	rootDev, err := storage.GetUnderlyingDevice()
	if err != nil {
		return err
	}

	if slices.Contains(zpool.Devices, rootDev) {
		return errors.New("list of devices includes the system root drive " + rootDev + ", which can't be used in other zpools")
	}

	// Ensure each device passed to `zpool create` is of the "by-id" format to be more resilient against changing device names.
	for i, dev := range zpool.Devices {
		zpool.Devices[i], err = storage.DeviceToID(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for i, dev := range zpool.Cache {
		zpool.Cache[i], err = storage.DeviceToID(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for i, dev := range zpool.Log {
		zpool.Log[i], err = storage.DeviceToID(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	if zpool.Special != nil {
		for i, dev := range zpool.Special.Devices {
			zpool.Special.Devices[i], err = storage.DeviceToID(ctx, dev, false)
			if err != nil {
				return err
			}
		}
	}

	// Generate a random encryption key.
	err = util.GenerateEncryptionKeyFile(keyfilePath)
	if err != nil {
		return err
	}

	// Create the ZFS pool.
	args := []string{"create", "-o", "ashift=12", "-O", "mountpoint=none", "-O", "encryption=aes-256-gcm", "-O", "keyformat=raw", "-O", "keylocation=file://" + keyfilePath, zpool.Name}

	switch zpool.Type {
	case "zfs-raid0":
		args = append(args, zpool.Devices...)
	case "zfs-raid1":
		args = append(args, "mirror")
		args = append(args, zpool.Devices...)
	case "zfs-raid10":
		middleIndex := len(zpool.Devices) / 2

		args = append(args, "mirror")
		args = append(args, zpool.Devices[:middleIndex]...)
		args = append(args, "mirror")
		args = append(args, zpool.Devices[middleIndex:]...)
	case "zfs-raidz1":
		args = append(args, "raidz1")
		args = append(args, zpool.Devices...)
	case "zfs-raidz2":
		args = append(args, "raidz2")
		args = append(args, zpool.Devices...)
	case "zfs-raidz3":
		args = append(args, "raidz3")
		args = append(args, zpool.Devices...)
	default:
		return errors.New("unsupported pool type " + zpool.Type)
	}

	if len(zpool.Cache) > 0 {
		args = append(args, "cache")
		args = append(args, zpool.Cache...)
	}

	if len(zpool.Log) > 0 {
		args = append(args, "log")
		if len(zpool.Log) > 1 {
			args = append(args, "mirror")
		}

		args = append(args, zpool.Log...)
	}

	if zpool.Special != nil && len(zpool.Special.Devices) > 0 {
		args = append(args, "special")

		switch zpool.Special.Type {
		case "zfs-raid0":
			args = append(args, zpool.Special.Devices...)
		case "zfs-raid1":
			args = append(args, "mirror")
			args = append(args, zpool.Special.Devices...)
		case "zfs-raid10":
			middleIndex := len(zpool.Special.Devices) / 2

			args = append(args, "mirror")
			args = append(args, zpool.Special.Devices[:middleIndex]...)
			args = append(args, "mirror")
			args = append(args, zpool.Special.Devices[middleIndex:]...)
		case "zfs-raidz1":
			args = append(args, "raidz1")
			args = append(args, zpool.Special.Devices...)
		case "zfs-raidz2":
			args = append(args, "raidz2")
			args = append(args, zpool.Special.Devices...)
		case "zfs-raidz3":
			args = append(args, "raidz3")
			args = append(args, zpool.Special.Devices...)
		default:
			return errors.New("unsupported special device type " + zpool.Type)
		}
	}

	err = createZpoolHelper(ctx, args, zpool.AllowMixedDevSizes)
	if err != nil {
		// Remove the encryption key file for the failed zpool.
		_ = os.Remove(keyfilePath)

		return err
	}

	// Reset encryption retrieval flag when a new zpool is created.
	s.System.Security.State.EncryptionRecoveryKeysRetrieved = false

	// If a special vdev is defined and SpecialSmallBlocksSizeInKB has a non-zero value, set the pool's special_small_blocks property.
	if zpool.Special != nil && zpool.Special.SpecialSmallBlocksSizeInKB != 0 {
		_, err := subprocess.RunCommandContext(ctx, "zfs", "set", "special_small_blocks="+strconv.Itoa(zpool.Special.SpecialSmallBlocksSizeInKB)+"K", zpool.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func createZpoolHelper(ctx context.Context, args []string, allowMixedDevSizes bool) error {
	_, err := subprocess.RunCommandContext(ctx, "zpool", args...)
	if err != nil {
		// If we're trying to create a new zpool with mixed device sizes, only allow if the user has specifically requested this.
		if strings.Contains(err.Error(), "contains devices of different sizes)") {
			if allowMixedDevSizes {
				args = append(args, "-f")
				_, err := subprocess.RunCommandContext(ctx, "zpool", args...)

				return err
			}

			return errors.New("refusing to create new zpool with devices of different sizes unless AllowMixedDevSizes is true")
		}

		return err
	}

	return nil
}

// DestroyZpool destroys an existing zpool.
func DestroyZpool(ctx context.Context, zpoolName string) error {
	// Don't allow destruction of the "local" zpool.
	if zpoolName == "local" {
		return errors.New("cannot destroy special zpool 'local'")
	}

	zpoolKey := "/var/lib/incus-os/zpool." + zpoolName + ".key"

	// Get a list of member devices.
	poolConfig, err := storage.GetZpoolMembers(ctx, zpoolName)
	if err != nil {
		// If we can't get the pool's config, it may have been removed outside of IncusOS' control.
		// If we still have an encryption key clean that up and return. Otherwise return an error
		// about the pool not existing.
		if strings.Contains(err.Error(), "cannot open '"+zpoolName+"': no such pool") {
			_, err := os.Stat(zpoolKey)
			if os.IsNotExist(err) {
				return errors.New("cannot destroy zpool '" + zpoolName + "': no such pool")
			}

			return os.Remove(zpoolKey)
		}

		return err
	}

	// Destroy the zpool.
	_, err = subprocess.RunCommandContext(ctx, "zpool", "destroy", zpoolName)
	if err != nil {
		return err
	}

	// Remove the old encryption key.
	err = os.Remove(zpoolKey)
	if err != nil {
		return err
	}

	// Wipe old member devices.
	for _, dev := range poolConfig.Devices {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.Log {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.Cache {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	if poolConfig.Special != nil {
		for _, dev := range poolConfig.Special.Devices {
			err := storage.WipeDrive(ctx, dev, false)
			if err != nil {
				return err
			}
		}
	}

	for _, dev := range poolConfig.DevicesDegraded {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.LogDegraded {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.CacheDegraded {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.SpecialDegraded {
		err := storage.WipeDrive(ctx, dev, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateZpool updates the devices used for an existing zpool.
func UpdateZpool(ctx context.Context, newConfig api.SystemStoragePool) error {
	// Check if the zpool exists.
	if !storage.PoolExists(ctx, newConfig.Name) {
		return errors.New("zpool '" + newConfig.Name + "' doesn't exist")
	}

	// Get the existing zpool config.
	currentConfig, err := storage.GetZpoolMembers(ctx, newConfig.Name)
	if err != nil {
		return err
	}

	// Verify we are given a supported type.
	if !slices.Contains(supportedPoolTypes, currentConfig.Type) {
		return errors.New("unsupported pool type " + currentConfig.Type)
	}

	// Verify the update contains at least as many device entries as exist in the current config.
	if len(newConfig.Devices) < len(currentConfig.Devices) {
		return fmt.Errorf("only %d devices provided in update, expected at least %d", len(newConfig.Devices), len(currentConfig.Devices))
	}

	if len(newConfig.Log) < len(currentConfig.Log) {
		return fmt.Errorf("only %d log devices provided in update, expected at least %d", len(newConfig.Log), len(currentConfig.Log))
	}

	if len(newConfig.Cache) < len(currentConfig.Cache) {
		return fmt.Errorf("only %d cache devices provided in update, expected at least %d", len(newConfig.Cache), len(currentConfig.Cache))
	}

	if newConfig.Special != nil {
		if len(newConfig.Special.Devices) < len(currentConfig.Special.Devices) {
			return fmt.Errorf("only %d special device devices provided in update, expected at least %d", len(newConfig.Special.Devices), len(currentConfig.Special.Devices))
		}
	}

	// Perform "local" pool specific checks.
	if newConfig.Name == "local" {
		// The only supported pool types are zfs-raid0 or zfs-raid1.
		if newConfig.Type != "zfs-raid0" && newConfig.Type != "zfs-raid1" {
			return errors.New("special zpool 'local' type must be either zfs-raid0 or zfs-raid1")
		}

		// Must have exactly one or two data devices and no log or cache.
		if len(newConfig.Devices) > 2 {
			return errors.New("special zpool 'local' cannot consist of more than two devices")
		}

		if len(newConfig.Log) > 0 {
			return errors.New("special zpool 'local' cannot have any log devices")
		}

		if len(newConfig.Cache) > 0 {
			return errors.New("special zpool 'local' cannot have any cache devices")
		}

		// The main system drive must ALWAYS be a member of the pool.
		rootDev, err := storage.GetUnderlyingDevice()
		if err != nil {
			return err
		}

		actualrootDev, err := storage.DeviceToID(ctx, rootDev, false)
		if err != nil {
			return err
		}

		if !slices.Contains(newConfig.Devices, actualrootDev+"-part11") {
			return errors.New("special zpool 'local' must always include main system partition '" + actualrootDev + "-part11'")
		}
	}

	// Apply updates.

	if currentConfig.Type == "zfs-raid0" && newConfig.Type == "zfs-raid1" {
		// Handle special case of converting a single device zpool to a mirrored zpool.
		err := convertPoolToMirror(ctx, currentConfig, newConfig)
		if err != nil {
			return err
		}
	} else {
		// Perform a normal update of the zpool's storage device(s).
		err := updateZpoolHelper(ctx, newConfig.Name, newConfig.Type, currentConfig.Devices, currentConfig.DevicesDegraded, newConfig.Devices)
		if err != nil {
			return err
		}
	}

	logType := "zfs-raid0"
	if len(newConfig.Log) > 1 {
		logType = "zfs-raid1"
	}

	err = updateZpoolHelper(ctx, newConfig.Name, logType, currentConfig.Log, currentConfig.LogDegraded, newConfig.Log)
	if err != nil {
		return err
	}

	err = updateZpoolHelper(ctx, newConfig.Name, "zfs-raid0", currentConfig.Cache, currentConfig.CacheDegraded, newConfig.Cache)
	if err != nil {
		return err
	}

	if newConfig.Special != nil {
		err := updateZpoolHelper(ctx, newConfig.Name, newConfig.Special.Type, currentConfig.Special.Devices, currentConfig.SpecialDegraded, newConfig.Special.Devices)
		if err != nil {
			return err
		}
	}

	return nil
}

func convertPoolToMirror(ctx context.Context, currentConfig api.SystemStoragePool, newConfig api.SystemStoragePool) error {
	// Basic checks
	if len(currentConfig.Devices) != 1 {
		return errors.New("cannot convert a pool with more than one device to a mirrored pool")
	}

	if len(newConfig.Devices) != 2 {
		return errors.New("converting to a mirrored pool requires exactly two devices")
	}

	// Get the device from the existing zpool.
	existingPoolDevice, err := storage.DeviceToID(ctx, currentConfig.Devices[0], false)
	if err != nil {
		return err
	}

	// Get the new device we'll use to convert this zpool into a mirror.
	newPoolDevice, err := storage.DeviceToID(ctx, newConfig.Devices[0], false)
	if err != nil {
		return err
	}

	if newPoolDevice == existingPoolDevice {
		newPoolDevice, err = storage.DeviceToID(ctx, newConfig.Devices[1], false)
		if err != nil {
			return err
		}
	}

	if newConfig.Name == "local" {
		newPoolDevice, err = partitionLocalPoolDevice(ctx, newPoolDevice)
		if err != nil {
			return err
		}
	}

	// Convert the pool to a mirror.
	_, err = subprocess.RunCommandContext(ctx, "zpool", "attach", newConfig.Name, existingPoolDevice, newPoolDevice)

	return err
}

// The "local" zpool is special. It is automatically created on first boot, but
// can be extended into a RAID0 or RAID1 configuration by adding another drive.
// When adding or replacing a device in this pool after converting to RAID1, we
// attempt to partition the second device in a similar fashion so the two underlying
// devices are the same size.
func partitionLocalPoolDevice(ctx context.Context, device string) (string, error) {
	// Create the partition at the correct offset
	_, err := subprocess.RunCommandContext(ctx, "sgdisk", "-n", "11:69826560:", device)
	if err != nil {
		return "", err
	}

	// Sleep for a bit, otherwise there's a race between udev updating symlinks and
	// ZFS trying to add the device to the pool.
	time.Sleep(500 * time.Millisecond)

	return device + "-part11", nil
}

func updateZpoolHelper(ctx context.Context, zpoolName string, zpoolType string, currentDevices []string, degradedDevices []string, newDevices []string) error {
	// Iterate over each degraded device. If the device itself is missing, this likely means
	// it was a failed drive that's already been physically removed. As such, it won't show up
	// in the storage API, but will still be recorded as part of the zpool. An end user will
	// intuitively attempt to replace the degraded device with a new one by adding another
	// existing drive to the zpool's configuration, which will result in an extra new device
	// here. Rather than using it to expand the zpool, which would still be degraded, instead
	// attempt to automatically replace the existing degraded device with the new one.
	for _, degradedDevice := range degradedDevices {
		// If no more extra devices are available, break out of the loop.
		if len(currentDevices) >= len(newDevices) {
			break
		}

		_, err := os.Stat(degradedDevice)
		if err != nil && os.IsNotExist(err) {
			// Add the missing degraded device to the list of current devices.
			// This will trigger a replace in the logic below.
			currentDevices = append(currentDevices, degradedDevice)
		}
	}

	// Compare the list of current devices to the list of new devices and apply updates as needed.
	for idx := range currentDevices {
		if newDevices[idx] == "" { //nolint:nestif
			// The update contains an empty string for this device -> remove from the pool.
			actualDev, err := storage.DeviceToID(ctx, currentDevices[idx], false)
			if err != nil {
				return err
			}

			zpoolCmd := "offline"
			if zpoolType == "zfs-raid0" {
				zpoolCmd = "remove"
			}

			_, err = subprocess.RunCommandContext(ctx, "zpool", zpoolCmd, zpoolName, actualDev)
			if err != nil {
				return err
			}

			if zpoolCmd == "remove" {
				err := storage.WipeDrive(ctx, actualDev, false)
				if err != nil {
					return err
				}
			}
		} else if newDevices[idx] != currentDevices[idx] {
			// The update contains a different device -> replace the existing device in the pool.
			actualDevOld, err := storage.DeviceToID(ctx, currentDevices[idx], false)
			if err != nil {
				return err
			}

			actualDevNew, err := storage.DeviceToID(ctx, newDevices[idx], false)
			if err != nil {
				return err
			}

			if zpoolName == "local" && zpoolType == "zfs-raid1" {
				actualDevNew, err = partitionLocalPoolDevice(ctx, actualDevNew)
				if err != nil {
					return err
				}
			}

			_, err = subprocess.RunCommandContext(ctx, "zpool", "replace", zpoolName, actualDevOld, actualDevNew)
			if err != nil {
				return err
			}

			err = storage.WipeDrive(ctx, actualDevOld, false)
			if err != nil {
				return err
			}
		}
	}

	// Add any remaining new devices to the zpool.
	devicesToAdd := newDevices[len(currentDevices):]

	if len(devicesToAdd) == 0 {
		// Can return here if there are no devices to add.
		return nil
	}

	if zpoolType == "zfs-raid0" { //nolint:nestif
		args := []string{"add", zpoolName}

		for _, dev := range devicesToAdd {
			actualDev, err := storage.DeviceToID(ctx, dev, false)
			if err != nil {
				return err
			}

			args = append(args, actualDev)
		}

		_, err := subprocess.RunCommandContext(ctx, "zpool", args...)
		if err != nil {
			return err
		}
	} else {
		actualNewDev, err := storage.DeviceToID(ctx, devicesToAdd[0], false)
		if err != nil {
			return err
		}

		// Bit of an edge case where a single "new" device might be an offlined member of the pool.
		// If so, try to bring it back online.
		if len(devicesToAdd) == 1 {
			_, err = subprocess.RunCommandContext(ctx, "zpool", "online", zpoolName, actualNewDev)
			if err == nil {
				// Successfully onlined the device, nothing else to do.
				return nil
			}
		}

		// Check that either one or two new devices are specified. raid10 requires two devices, one for each mirror vdev
		// while raid1 and raidz requires a single device when expanding. Because we return immediately, we only support
		// incremental expansion of an existing pool because a resilver event must complete afterwards.
		if zpoolType == "zfs-raid10" {
			if len(devicesToAdd) != 2 {
				return errors.New("expanding a raid10 pool requires a pair of new devices")
			}
		} else {
			if len(devicesToAdd) != 1 {
				return errors.New("expanding a pool can only be performed one device at a time, due to a required resilver between expansions")
			}
		}

		// Get the current list of devices that comprise this pool.
		currentConfig, err := storage.GetZpoolMembers(ctx, zpoolName)
		if err != nil {
			return err
		}

		attachDevice := currentConfig.Devices[0]
		if strings.HasPrefix(zpoolType, "zfs-raidz") {
			attachDevice = strings.TrimPrefix(zpoolType, "zfs-") + "-0"
		}

		// Expand the storage pool.
		_, err = subprocess.RunCommandContext(ctx, "zpool", "attach", zpoolName, attachDevice, actualNewDev)
		if err != nil {
			return err
		}

		if zpoolType == "zfs-raid10" {
			secondNewDev, err := storage.DeviceToID(ctx, devicesToAdd[1], false)
			if err != nil {
				return err
			}

			_, err = subprocess.RunCommandContext(ctx, "zpool", "attach", zpoolName, currentConfig.Devices[len(currentConfig.Devices)-1], secondNewDev)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetZpoolEncryptionKeys returns a map of base64-encoded encryption keys for all local zpools.
func GetZpoolEncryptionKeys() (map[string]string, error) {
	ret := make(map[string]string)

	files, err := os.ReadDir("/var/lib/incus-os/")
	if err != nil {
		return ret, err
	}

	for _, file := range files {
		filename := file.Name()

		if strings.HasPrefix(filename, "zpool.") && strings.HasSuffix(filename, ".key") {
			// #nosec G304
			contents, err := os.ReadFile(filepath.Join("/var/lib/incus-os/", filename))
			if err != nil {
				return ret, err
			}

			zpool := strings.TrimPrefix(strings.TrimSuffix(filename, ".key"), "zpool.")

			ret[zpool] = base64.StdEncoding.EncodeToString(contents)
		}
	}

	return ret, nil
}

// ImportExistingPool will import an existing but currently unmanaged ZFS pool.
// After importing, it will save and then load the encryption key.
func ImportExistingPool(ctx context.Context, pool string, key string) error {
	reverter := revert.New()
	defer reverter.Fail()

	// Import the existing pool.
	_, err := subprocess.RunCommandContext(ctx, "zpool", "import", pool)
	if err != nil {
		return err
	}

	reverter.Add(func() {
		_, _ = subprocess.RunCommandContext(ctx, "zpool", "export", "-f", pool)
	})

	// Make sure the pool is encrypted.
	encryptionStatus, err := subprocess.RunCommandContext(ctx, "zfs", "get", "encryption", "-H", "-o", "value", pool)
	if err != nil {
		return err
	}

	if strings.TrimSpace(encryptionStatus) == "off" {
		return errors.New("refusing to import unencrypted ZFS pool")
	}

	// Make sure the pool is uses a raw key.
	keyFormat, err := subprocess.RunCommandContext(ctx, "zfs", "get", "keyformat", "-H", "-o", "value", pool)
	if err != nil {
		return err
	}

	if strings.TrimSpace(keyFormat) != "raw" {
		return errors.New("refusing to import pool that doesn't use a raw encryption key")
	}

	keyfilePath := "/var/lib/incus-os/zpool." + pool + ".key"

	// Decode encryption key into raw bytes.
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

	reverter.Add(func() {
		_ = os.Remove(keyfilePath)
	})

	// Make sure the new pool knows where to find the encryption key.
	_, err = subprocess.RunCommandContext(ctx, "zfs", "set", "keylocation=file://"+keyfilePath, pool)
	if err != nil {
		return err
	}

	// Load the pool's encryption key.
	_, err = subprocess.RunCommandContext(ctx, "zfs", "load-key", pool)
	if err != nil {
		return err
	}

	reverter.Success()

	return nil
}

// CreateDataset creates a new dataset in the specified pool and applies some optional properties.
func CreateDataset(ctx context.Context, poolName string, name string, properties map[string]string) error {
	args := []string{"create", poolName + "/" + name} //nolint:prealloc

	for k, v := range properties {
		args = append(args, "-o", k+"="+v)
	}

	_, err := subprocess.RunCommandContext(ctx, "zfs", args...)

	return err
}

// DestroyDataset removes a dataset from the specified pool.
func DestroyDataset(ctx context.Context, poolName string, name string, force bool) error {
	args := []string{"destroy", poolName + "/" + name}
	if force {
		args = append(args, "-R")
	}

	_, err := subprocess.RunCommandContext(ctx, "zfs", args...)

	return err
}

// ScrubZpool scrubs the specified pool.
func ScrubZpool(ctx context.Context, poolName string) error {
	// Check if the zpool exists.
	if !storage.PoolExists(ctx, poolName) {
		return errors.New("zpool '" + poolName + "' doesn't exist")
	}

	info, err := storage.GetStorageInfo(ctx)
	if err != nil {
		return err
	}

	pool := api.SystemStoragePool{}

	for _, p := range info.Pools {
		if p.Name == poolName {
			pool = p
		}
	}

	if pool.LastScrub != nil && pool.LastScrub.State == api.ScrubInProgress {
		return storage.ErrScrubAlreadyInProgress
	}

	// Perform the scrub.
	_, err = subprocess.RunCommandContext(ctx, "zpool", "scrub", poolName)
	if err != nil {
		return err
	}

	return nil
}

// ScrubAllPools scrubs all pools in the system sequentially, blocking until the scrub is complete.
func ScrubAllPools(ctx context.Context) error {
	info, err := storage.GetStorageInfo(ctx)
	if err != nil {
		return err
	}

	// Scrub every pool sequentially.
	for _, pool := range info.Pools {
		slog.InfoContext(ctx, "Scrubbing pool", slog.String("pool", pool.Name))

		// If a scrub is already in progress for a pool, skip it.
		if pool.LastScrub != nil && pool.LastScrub.State == api.ScrubInProgress {
			continue
		}

		// Perform the scrub.
		_, err = subprocess.RunCommandContext(ctx, "zpool", "scrub", pool.Name)
		if err != nil {
			return err
		}

		// Wait for the scrub to finish.
		for {
			latestInfo, err := storage.GetStorageInfo(ctx)
			if err != nil {
				return err
			}

			latestPoolInfo := api.SystemStoragePool{}

			for _, p := range latestInfo.Pools {
				if p.Name == pool.Name {
					latestPoolInfo = p
				}
			}

			// If the scrub is not in progress, break and move to the next pool.
			if latestPoolInfo.LastScrub != nil && latestPoolInfo.LastScrub.State != api.ScrubInProgress {
				break
			}

			time.Sleep(time.Minute)
		}
	}

	return nil
}
