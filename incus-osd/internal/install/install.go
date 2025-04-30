package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/tui"
)

// Install holds information necessary to perform an installation.
type Install struct {
	config *seed.InstallSeed
	tui    *tui.TUI
}

// IsInstallNeeded checks for the presence of an install.{json,yaml} file in the
// seed partition to indicate if we should attempt to install incus-osd to a local disk.
func IsInstallNeeded() bool {
	_, err := seed.GetInstallConfig(seed.SeedPartitionPath)

	// If we have any empty install file, that should still trigger an install.
	if errors.Is(err, io.EOF) {
		return true
	}

	return err == nil
}

// NewInstall returns a new Install object with its configuration, if any, populated from the seed partition.
func NewInstall(t *tui.TUI) (*Install, error) {
	ret := &Install{
		tui: t,
	}

	var err error
	ret.config, err = seed.GetInstallConfig(seed.SeedPartitionPath)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	return ret, nil
}

// DoInstall performs the necessary steps for installing incus-osd to a local disk.
func (i *Install) DoInstall(ctx context.Context) error {
	slog.Info("Starting install of incus-osd to local disk")
	i.tui.DisplayModal("Incus OS Install", "Starting install of incus-osd to local disk.", 0, 0)

	sourceDevice, sourceIsReadonly, err := i.getSourceDevice()
	if err != nil {
		i.tui.DisplayModal("Incus OS Install", "[red]Error: "+err.Error(), 0, 0)

		return err
	}

	targetDevice, err := i.getTargetDevice(ctx, sourceDevice)
	if err != nil {
		i.tui.DisplayModal("Incus OS Install", "[red]Error: "+err.Error(), 0, 0)

		return err
	}

	slog.Info("Installing incus-osd", "source", sourceDevice, "target", targetDevice)
	i.tui.DisplayModal("Incus OS Install", fmt.Sprintf("Installing incus-osd from %s to %s.", sourceDevice, targetDevice), 0, 0)

	err = i.performInstall(ctx, sourceDevice, targetDevice, sourceIsReadonly)
	if err != nil {
		i.tui.DisplayModal("Incus OS Install", "[red]Error: "+err.Error(), 0, 0)

		return err
	}

	slog.Info("Incus OS was successfully installed")
	slog.Info("Please remove the install media to complete the installation")
	i.tui.DisplayModal("Incus OS Install", "Incus OS was successfully installed.\nPlease remove the install media to complete the installation.", 0, 0)

	return i.rebootUponDeviceRemoval(ctx, sourceDevice)
}

// getSourceDevice determines the underlying device incus-osd is running on and if it is read-only.
func (*Install) getSourceDevice() (string, bool, error) {
	// Start by determining the underlying device that /boot/EFI is on.
	s := unix.Stat_t{}
	err := unix.Stat("/boot/EFI", &s)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Check if we're running from a CDROM.
			err = unix.Stat("/dev/sr0", &s)
			if err == nil {
				return "/dev/mapper/sr0", true, nil
			}
		}

		return "", false, err
	}

	// Test if we're on a read-only file system.
	isReadonlyInstallFS := false
	f, err := os.Create("/boot/EFI/testfile")
	switch {
	case err == nil:
		_ = f.Close()
		_ = os.Remove("/boot/EFI/testfile")
	case strings.Contains(err.Error(), "read-only file system"):
		isReadonlyInstallFS = true
	default:
		return "", false, err
	}

	major := unix.Major(s.Dev)
	minor := unix.Minor(s.Dev)
	rootDev := fmt.Sprintf("%d:%d\n", major, minor)

	// Get a list of all the block devices.
	entries, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return "", isReadonlyInstallFS, err
	}

	// Iterate through each of the block devices until we find the one for /boot/EFI.
	for _, entry := range entries {
		entryPath := filepath.Join("/sys/class/block", entry.Name())

		dev, err := os.ReadFile(filepath.Join(entryPath, "dev")) //nolint:gosec
		if err != nil {
			continue
		}

		// We've found the device.
		if string(dev) == rootDev {
			// Read the symlink for the device, which will end with something like "/block/sda/sda1".
			path, err := os.Readlink(entryPath)
			if err != nil {
				return "", isReadonlyInstallFS, err
			}

			// Drop the last element of the path (the partition), then get the base of the resulting path (the actual device).
			parentDir, _ := filepath.Split(path)
			underlyingDev := filepath.Base(parentDir)

			return "/dev/" + underlyingDev, isReadonlyInstallFS, nil
		}
	}

	return "", isReadonlyInstallFS, errors.New("unable to determine source device")
}

// getTargetDevice determines the underlying device to install incus-osd on.
func (i *Install) getTargetDevice(ctx context.Context, sourceDevice string) (string, error) {
	type blockdevices struct {
		KName string `json:"kname"`
		ID    string `json:"id-link"` //nolint:tagliatelle
	}

	type lsblkOutput struct {
		Blockdevices []blockdevices `json:"blockdevices"`
	}

	potentialTargets := []blockdevices{}

	// Get NVME drives first.
	nvmeTargets := lsblkOutput{}
	output, err := subprocess.RunCommandContext(ctx, "lsblk", "-N", "-iJnp", "-o", "KNAME,ID_LINK")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(output), &nvmeTargets)
	if err != nil {
		return "", err
	}

	potentialTargets = append(potentialTargets, nvmeTargets.Blockdevices...)

	// Get SCSI drives second.
	scsiTargets := lsblkOutput{}
	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-S", "-iJnp", "-o", "KNAME,ID_LINK")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(output), &scsiTargets)
	if err != nil {
		return "", err
	}

	potentialTargets = append(potentialTargets, scsiTargets.Blockdevices...)

	// Get virtual drives last.
	virtualTargets := lsblkOutput{}
	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-v", "-iJnp", "-o", "KNAME,ID_LINK")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(output), &virtualTargets)
	if err != nil {
		return "", err
	}

	potentialTargets = append(potentialTargets, virtualTargets.Blockdevices...)

	// Ensure we found at least two devices (the install device and potential install device(s)). If no Target
	// configuration was found, only proceed if exactly two devices were found.
	if len(potentialTargets) < 2 {
		return "", errors.New("no potential install devices found")
	} else if i.config.Target == nil && len(potentialTargets) != 2 {
		return "", errors.New("no target configuration provided, and didn't find exactly one install device")
	}

	// Loop through all disks, selecting the first one that isn't the source and matches the Target configuration.
	for _, device := range potentialTargets {
		if device.KName == sourceDevice {
			continue
		}

		if i.config.Target == nil || strings.Contains(device.ID, i.config.Target.ID) {
			return device.KName, nil
		}
	}

	return "", errors.New("unable to determine target device")
}

// performInstall performs the steps to install incus-osd from the given target to the source device.
func (i *Install) performInstall(ctx context.Context, sourceDevice string, targetDevice string, sourceIsReadonly bool) error {
	// Verify the target device doesn't already have a partition table, or that `ForceInstall` is set to true.
	output, err := subprocess.RunCommandContext(ctx, "sgdisk", "-v", targetDevice)
	if err != nil {
		return err
	}

	if !strings.Contains(output, "Creating new GPT entries in memory") && !i.config.ForceInstall {
		return fmt.Errorf("a partition table already exists on device '%s', and `ForceInstall` from install configuration isn't true", targetDevice)
	}

	// Turn off swap and unmount /boot.
	_, err = subprocess.RunCommandContext(ctx, "swapoff", "-a")
	if err != nil {
		return err
	}

	err = unix.Unmount("/boot/", 0)
	if err != nil {
		// /boot/ won't exist when installer is running from a CDROM.
		if !errors.Is(err, os.ErrNotExist) || !sourceIsReadonly {
			return err
		}
	}

	// Number of partitions to copy.
	numPartitionsToCopy := 8
	if sourceIsReadonly {
		numPartitionsToCopy = 5
	}

	// Copy partition definitions to target device. We can't just do a `sgdisk -R target source`
	// because the install media may have a different sector size than the target device (for example,
	// if the installer is running from a CDROM).
	copyPartitionDefinition := func(src string, tgt string, partitionIndex int) error {
		// Get source partition information.
		output, err := subprocess.RunCommandContext(ctx, "sgdisk", "-i", strconv.Itoa(partitionIndex), src)
		if err != nil {
			return err
		}

		partitionTypeRegex := regexp.MustCompile(`Partition GUID code: .+ \((.+)\)`)
		partitionGUIDRegex := regexp.MustCompile(`Partition unique GUID: (.+)`)
		partitionNameRegex := regexp.MustCompile(`Partition name: '(.+)'`)
		partitionSizeRegex := regexp.MustCompile(`Partition size: \d+ sectors \((.+)\)`)

		partitionHexCode := ""
		partitionType := partitionTypeRegex.FindStringSubmatch(output)[1]
		partitionGUID := partitionGUIDRegex.FindStringSubmatch(output)[1]
		partitionName := partitionNameRegex.FindStringSubmatch(output)[1]
		partitionSize := strings.ReplaceAll(partitionSizeRegex.FindStringSubmatch(output)[1], " ", "")

		switch partitionType {
		case "EFI system partition":
			partitionHexCode = "EF00"
		case "Linux filesystem":
			partitionHexCode = "8300"
		case "Linux x86-64 /usr verity signature":
			partitionHexCode = "8385"
		case "Linux x86-64 /usr verity":
			partitionHexCode = "8319"
		case "Linux x86-64 /usr":
			partitionHexCode = "8314"
		}

		// Create the partition on the target device.
		_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", strconv.Itoa(partitionIndex)+"::+"+partitionSize, "-u", strconv.Itoa(partitionIndex)+":"+partitionGUID, "-t", strconv.Itoa(partitionIndex)+":"+partitionHexCode, "-c", strconv.Itoa(partitionIndex)+":"+partitionName, tgt)

		return err
	}

	// If we're running from a CDROM, fixup the actual device we should look at for the partitions.
	actualSourceDevice := sourceDevice
	if actualSourceDevice == "/dev/mapper/sr0" {
		actualSourceDevice = "/dev/sr0"
	}

	// Copy partition definitions.
	for i := 1; i <= numPartitionsToCopy; i++ {
		err := copyPartitionDefinition(actualSourceDevice, targetDevice, i)
		if err != nil {
			return err
		}
	}

	// Get partition prefixes, if needed.
	sourcePartitionPrefix := getPartitionPrefix(sourceDevice)
	targetPartitionPrefix := getPartitionPrefix(targetDevice)

	doCopy := func(partitionIndex int) error {
		sourcePartition, err := os.OpenFile(fmt.Sprintf("%s%s%d", sourceDevice, sourcePartitionPrefix, partitionIndex), os.O_RDONLY, 0o0600)
		if err != nil {
			return err
		}
		defer sourcePartition.Close()

		partitionSize, err := sourcePartition.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}

		_, err = sourcePartition.Seek(0, 0)
		if err != nil {
			return err
		}

		targetPartition, err := os.OpenFile(fmt.Sprintf("%s%s%d", targetDevice, targetPartitionPrefix, partitionIndex), os.O_WRONLY, 0o0600)
		if err != nil {
			return err
		}
		defer targetPartition.Close()

		// Copy data in 1MiB chunks.
		count := int64(0)
		for {
			_, err := io.CopyN(targetPartition, sourcePartition, 1024*1024)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return err
			}

			if count%10 == 0 {
				i.tui.DisplayModal("Incus OS Install", fmt.Sprintf("Copying partition %d of %d.", partitionIndex, numPartitionsToCopy), count*1024*1024, partitionSize)
			}
			count++
		}

		return nil
	}

	// Copy the partition contents.
	for i := 1; i <= numPartitionsToCopy; i++ {
		err := doCopy(i)
		if err != nil {
			return err
		}
	}

	// Remove the install configuration file, if present, from the target seed partition.
	targetSeedPartition := fmt.Sprintf("%s%s2", targetDevice, targetPartitionPrefix)
	for _, filename := range []string{"install.json", "install.yaml"} {
		_, err = subprocess.RunCommandContext(ctx, "tar", "-f", targetSeedPartition, "--delete", filename)
		if err != nil && !strings.Contains(err.Error(), fmt.Sprintf("tar: %s: Not found in archive", filename)) {
			return err
		}
	}

	return nil
}

// rebootUponDeviceRemoval waits for the given device to disappear from /dev/, and once it does
// it will reboot the system. If ForceReoot is true in the config, the system will reboot immediately.
func (i *Install) rebootUponDeviceRemoval(_ context.Context, device string) error {
	partition := fmt.Sprintf("%s%s1", device, getPartitionPrefix(device))

	// Wait for the partition to disappear; if ForceReboot is true, skip the loop and immediately reboot.
	for !i.config.ForceReboot {
		_, err := os.Stat(partition)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}

			return err
		}

		time.Sleep(1 * time.Second)
	}

	// Do a final sync and then reboot the system.
	unix.Sync()

	return os.WriteFile("/proc/sysrq-trigger", []byte("b"), 0o600)
}

// getPartitionPrefix returns the necessary partition prefix, if any, for a give device.
// nvme devices have partitions named "pN", while traditional disk partitions are just "N".
func getPartitionPrefix(device string) string {
	if strings.Contains(device, "/nvme") || strings.Contains(device, "mapper/sr0") {
		return "p"
	}

	return ""
}
