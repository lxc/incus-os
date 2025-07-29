package zfs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
)

// ImportOrCreateLocalPool imports and loads the encryption key for the "local" ZFS pool if the it
// exists, otherwise will create an encrypted ZFS pool "local" in the partition labeled "local-data".
func ImportOrCreateLocalPool(ctx context.Context, s *state.State) error {
	// Check if the "local" ZFS pool exists.
	_, err := subprocess.RunCommandContext(ctx, "zpool", "import", "local")
	if err == nil || strings.Contains(err.Error(), "cannot import 'local': a pool with that name already exists") {
		// Pool is available, now load the encryption key and we're done.
		_, err := subprocess.RunCommandContext(ctx, "zfs", "load-key", "local")
		if err != nil && !strings.Contains(err.Error(), "Key load error: Key already loaded for 'local'.") {
			return err
		}

		return nil
	} else if strings.Contains(err.Error(), "cannot import 'local': no such pool available") {
		// Need to create the "local" ZFS pool.
		zpool := api.SystemStoragePool{
			Name:    "local",
			Type:    "zfs-raid0",
			Devices: []string{"/dev/disk/by-partlabel/local-data"},
		}

		return CreateZpool(ctx, zpool, s)
	}

	return err
}

// PoolExists checks if a given ZFS pool exists.
func PoolExists(ctx context.Context, zpoolName string) bool {
	_, err := subprocess.RunCommandContext(ctx, "zpool", "status", zpoolName)

	return err == nil
}

// CreateZpool creates a new zpool.
func CreateZpool(ctx context.Context, zpool api.SystemStoragePool, s *state.State) error {
	keyfilePath := "/var/lib/incus-os/zpool." + zpool.Name + ".key"

	// Verify a zpool name was provided.
	if zpool.Name == "" {
		return errors.New("a name for the zpool must be provided")
	}

	// Check if the zpool already exists.
	if PoolExists(ctx, zpool.Name) {
		return errors.New("zpool '" + zpool.Name + "' already exists")
	}

	// Verify we are given a supported type.
	if !slices.Contains([]string{"zfs-raid0", "zfs-raid1", "zfs-raid10", "zfs-raidz1", "zfs-raidz2", "zfs-raidz3"}, zpool.Type) {
		return errors.New("unsupported pool type " + zpool.Type)
	}

	// Verify at least one device was specified.
	if len(zpool.Devices) == 0 {
		return errors.New("at least one device must be specified")
	}

	// If asked to create a raid10 pool, ensure an even number of devices greater than or equal to four was specified.
	if zpool.Type == "zfs-raid10" {
		if len(zpool.Devices) < 4 {
			return errors.New("at least four devices must be specified when creating a raid10 pool")
		} else if len(zpool.Devices)%2 != 0 {
			return errors.New("an even number of devices must be specified when creating a raid10 pool")
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
		zpool.Devices[i], err = storage.DeviceToID(ctx, dev)
		if err != nil {
			return err
		}
	}

	for i, dev := range zpool.Cache {
		zpool.Cache[i], err = storage.DeviceToID(ctx, dev)
		if err != nil {
			return err
		}
	}

	for i, dev := range zpool.Log {
		zpool.Log[i], err = storage.DeviceToID(ctx, dev)
		if err != nil {
			return err
		}
	}

	// Generate a random encryption key.
	devUrandom, err := os.OpenFile("/dev/urandom", os.O_RDONLY, 0o0600)
	if err != nil {
		return err
	}
	defer devUrandom.Close()

	// #nosec G304
	keyfile, err := os.OpenFile(keyfilePath, os.O_CREATE|os.O_WRONLY, 0o0600)
	if err != nil {
		return err
	}
	defer keyfile.Close()

	count, err := io.CopyN(keyfile, devUrandom, 32)
	if err != nil {
		return err
	}

	if count != 32 {
		// Remove the bad encryption key file, if it exists.
		_ = os.Remove(keyfilePath)

		return errors.New("failed to read 32 bytes from /dev/urandom")
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
		args = append(args, zpool.Log...)
	}

	_, err = subprocess.RunCommandContext(ctx, "zpool", args...)
	if err != nil {
		// Remove the encryption key file for the failed zpool.
		_ = os.Remove(keyfilePath)

		return err
	}

	// Reset encryption retrieval flag when a new zpool is created.
	s.System.Security.State.EncryptionRecoveryKeysRetrieved = false

	return nil
}

// DestroyZpool destroys an existing zpool.
func DestroyZpool(ctx context.Context, zpoolName string) error {
	// Don't allow destruction of the "local" zpool.
	if zpoolName == "local" {
		return errors.New("cannot destroy special zpool 'local'")
	}

	// Get a list of member devices.
	poolConfig, err := storage.GetZpoolMembers(ctx, zpoolName)
	if err != nil {
		return err
	}

	// Destroy the zpool.
	_, err = subprocess.RunCommandContext(ctx, "zpool", "destroy", zpoolName)
	if err != nil {
		return err
	}

	// Remove the old encryption key.
	err = os.Remove("/var/lib/incus-os/zpool." + zpoolName + ".key")
	if err != nil {
		return err
	}

	// Wipe old member devices.
	for _, dev := range poolConfig.Devices {
		_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", dev)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.Log {
		_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", dev)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.Cache {
		_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", dev)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.DevicesDegraded {
		_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", dev)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.LogDegraded {
		_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", dev)
		if err != nil {
			return err
		}
	}

	for _, dev := range poolConfig.CacheDegraded {
		_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", dev)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateZpool updates the devices used for an existing zpool.
func UpdateZpool(ctx context.Context, newConfig api.SystemStoragePool) error {
	// Check if the zpool exists.
	if !PoolExists(ctx, newConfig.Name) {
		return errors.New("zpool '" + newConfig.Name + "' doesn't exist")
	}

	// Don't allow modification of the "local" zpool.
	if newConfig.Name == "local" {
		return errors.New("cannot update special zpool 'local'")
	}

	// Get the existing zpool config.
	currentConfig, err := storage.GetZpoolMembers(ctx, newConfig.Name)
	if err != nil {
		return err
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

	vdevName := ""

	switch newConfig.Type {
	case "zfs-raid0":
	case "zfs-raid1", "zfs-raid10":
		vdevName = "mirror"
	case "zfs-raidz1":
		vdevName = "raidz1-0"
	case "zfs-raidz2":
		vdevName = "raidz2-0"
	case "zfs-raidz3":
		vdevName = "raidz3-0"
	default:
		return errors.New("unsupported pool type " + newConfig.Type)
	}

	// Apply updates.
	err = updateZpoolHelper(ctx, newConfig.Name, vdevName, currentConfig.Devices, newConfig.Devices)
	if err != nil {
		return err
	}

	err = updateZpoolHelper(ctx, newConfig.Name, "log", currentConfig.Log, newConfig.Log)
	if err != nil {
		return err
	}

	err = updateZpoolHelper(ctx, newConfig.Name, "cache", currentConfig.Cache, newConfig.Cache)
	if err != nil {
		return err
	}

	return nil
}

func updateZpoolHelper(ctx context.Context, zpoolName string, vdevName string, currentDevices []string, newDevices []string) error {
	// Compare the two lists of devices and apply updates as needed.
	for idx := range currentDevices {
		if newDevices[idx] == "" { //nolint:nestif
			// The update contains an empty string for this device -> remove from the pool.
			actualDev, err := storage.DeviceToID(ctx, currentDevices[idx])
			if err != nil {
				return err
			}

			zpoolCmd := "remove"

			if vdevName == "mirror" || strings.HasPrefix(vdevName, "raidz") {
				zpoolCmd = "offline"
			}

			_, err = subprocess.RunCommandContext(ctx, "zpool", zpoolCmd, zpoolName, actualDev)
			if err != nil {
				return err
			}

			if zpoolCmd == "remove" {
				_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", actualDev)
				if err != nil {
					return err
				}
			}
		} else if newDevices[idx] != currentDevices[idx] {
			// The update contains a different device -> replace the existing device in the pool.
			actualDevOld, err := storage.DeviceToID(ctx, currentDevices[idx])
			if err != nil {
				return err
			}

			actualDevNew, err := storage.DeviceToID(ctx, newDevices[idx])
			if err != nil {
				return err
			}

			_, err = subprocess.RunCommandContext(ctx, "zpool", "replace", zpoolName, actualDevOld, actualDevNew)
			if err != nil {
				return err
			}

			_, err = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", actualDevOld)
			if err != nil {
				return err
			}
		}
	}

	// Add any additional new devices.
	if vdevName == "mirror" { //nolint:nestif
		if len(newDevices)-len(currentDevices) == 1 {
			// Bit of an edge case where a single "new" device might be an offlined member of the mirror. As such, it's still part
			// of the pool, so try to bring it back online.
			actualDev, err := storage.DeviceToID(ctx, newDevices[len(currentDevices)])
			if err != nil {
				return err
			}

			_, err = subprocess.RunCommandContext(ctx, "zpool", "online", zpoolName, actualDev)
			if err != nil {
				// If we couldn't online the device, then it's brand new, but ZFS requires at least two devices when adding a new mirror.
				return errors.New("adding to a mirror requires at least two devices")
			}
		} else if len(currentDevices) < len(newDevices) {
			args := []string{"add", zpoolName, "mirror"}

			for _, dev := range newDevices[len(currentDevices):] {
				actualDev, err := storage.DeviceToID(ctx, dev)
				if err != nil {
					return err
				}

				args = append(args, actualDev)
			}

			_, err := subprocess.RunCommandContext(ctx, "zpool", args...)
			if err != nil {
				return err
			}
		}
	} else {
		for idx := len(currentDevices); idx < len(newDevices); idx++ {
			args := []string{}

			if vdevName == "" || vdevName == "log" || vdevName == "cache" {
				args = append(args, "add")
			} else {
				args = append(args, "attach")

				// Expanding a vdev triggers a scrub/expand, during which time we can't add an additional device.
				// Rather than a somewhat cryptic error, return a nicer message.
				if len(newDevices)-len(currentDevices) > 1 {
					return errors.New("expanding a pool by more than one device at a time isn't supported, due to necessary array resync")
				}
			}

			args = append(args, zpoolName)

			if vdevName != "" {
				args = append(args, vdevName)
			}

			actualDev, err := storage.DeviceToID(ctx, newDevices[idx])
			if err != nil {
				return err
			}

			args = append(args, actualDev)

			_, err = subprocess.RunCommandContext(ctx, "zpool", args...)
			if err != nil {
				if !strings.Contains(err.Error(), actualDev+"-part1 is part of active pool '"+zpoolName+"'") || !strings.HasPrefix(vdevName, "raidz") {
					return err
				}

				// Bit of an edge case where the "new" device might be an offlined member of the raidz. As such, it's still part
				// of the pool, so try to bring it back online.
				_, newErr := subprocess.RunCommandContext(ctx, "zpool", "online", zpoolName, actualDev)
				if newErr != nil {
					// Return the original error, so we don't confuse the user with a failed "online" command.
					return err
				}
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
