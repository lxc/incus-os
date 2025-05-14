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
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/tui"
)

// Install holds information necessary to perform an installation.
type Install struct {
	config *seed.InstallSeed
	tui    *tui.TUI
}

type blockdevices struct {
	KName string `json:"kname"`
	ID    string `json:"id-link"` //nolint:tagliatelle
}

type lsblkOutput struct {
	Blockdevices []blockdevices `json:"blockdevices"`
}

var cdromMappedDevice = "/dev/mapper/sr0"

// CheckSystemRequirements verifies that the system meets the minimum requirements for running Incus OS.
func CheckSystemRequirements(ctx context.Context) error {
	// Check if systemd-repart has failed (we're either running from read-only media or a
	// small USB stick) which normally indicates we're about to start an install and there's
	// no install seed present.
	if systemd.IsFailed(ctx, "systemd-repart") && !ShouldPerformInstall() {
		return errors.New("unable to begin install without seed configuration")
	}

	// Check if a TPM device is present.
	_, err := os.Stat("/dev/tpm0")
	if err != nil {
		return errors.New("no TPM device found")
	}

	// Perform install-specific checks.
	if ShouldPerformInstall() {
		// Check that we have either been told what target device to use, or that we can automatically figure it out.
		source, _, err := getSourceDevice(ctx)
		if err != nil {
			return err
		}

		targets, err := getAllTargets(ctx)
		if err != nil {
			return err
		}

		config, err := seed.GetInstallConfig(seed.SeedPartitionPath)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		_, err = getTargetDevice(targets, config.Target, source)
		if err != nil {
			devices := []string{}
			for _, t := range targets {
				devices = append(devices, t.ID)
			}

			return errors.New(err.Error() + " (detected devices: " + strings.Join(devices, ", ") + ")")
		}
	}

	return nil
}

// ShouldPerformInstall checks for the presence of an install.{json,yaml} file in the
// seed partition to indicate if we should attempt to install incus-osd to a local disk.
func ShouldPerformInstall() bool {
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

	sourceDevice, sourceIsReadonly, err := getSourceDevice(ctx)
	if err != nil {
		i.tui.DisplayModal("Incus OS Install", "[red]Error: "+err.Error(), 0, 0)

		return err
	}

	targets, err := getAllTargets(ctx)
	if err != nil {
		i.tui.DisplayModal("Incus OS Install", "[red]Error: "+err.Error(), 0, 0)

		return err
	}

	targetDevice, err := getTargetDevice(targets, i.config.Target, sourceDevice)
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
func getSourceDevice(ctx context.Context) (string, bool, error) {
	// Start by determining the underlying device that /boot/EFI is on.
	s := unix.Stat_t{}
	err := unix.Stat("/boot/EFI", &s)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Check if we're running from a CDROM.
			err = unix.Stat("/dev/sr0", &s)
			if err == nil {
				return cdromMappedDevice, true, nil
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

	if systemd.IsFailed(ctx, "systemd-repart") {
		isReadonlyInstallFS = true
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

// getAllTargets returns a list of all potential install target devices.
func getAllTargets(ctx context.Context) ([]blockdevices, error) {
	ret := []blockdevices{}

	// Get NVME drives first.
	nvmeTargets := lsblkOutput{}
	output, err := subprocess.RunCommandContext(ctx, "lsblk", "-N", "-iJnp", "-o", "KNAME,ID_LINK")
	if err != nil {
		return []blockdevices{}, err
	}

	err = json.Unmarshal([]byte(output), &nvmeTargets)
	if err != nil {
		return []blockdevices{}, err
	}

	ret = append(ret, nvmeTargets.Blockdevices...)

	// Get SCSI drives second.
	scsiTargets := lsblkOutput{}
	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-S", "-iJnp", "-o", "KNAME,ID_LINK")
	if err != nil {
		return []blockdevices{}, err
	}

	err = json.Unmarshal([]byte(output), &scsiTargets)
	if err != nil {
		return []blockdevices{}, err
	}

	ret = append(ret, scsiTargets.Blockdevices...)

	// Get virtual drives last.
	virtualTargets := lsblkOutput{}
	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-v", "-iJnp", "-o", "KNAME,ID_LINK")
	if err != nil {
		return []blockdevices{}, err
	}

	err = json.Unmarshal([]byte(output), &virtualTargets)
	if err != nil {
		return []blockdevices{}, err
	}

	ret = append(ret, virtualTargets.Blockdevices...)

	return ret, nil
}

// getTargetDevice determines the underlying device to install incus-osd on.
func getTargetDevice(potentialTargets []blockdevices, seedTarget *seed.InstallSeedTarget, sourceDevice string) (string, error) {
	// Ensure we found at least two devices (the install device and potential install device(s)). If no Target
	// configuration was found, only proceed if exactly two devices were found.
	if len(potentialTargets) < 2 {
		return "", errors.New("no potential install devices found")
	} else if seedTarget == nil && len(potentialTargets) != 2 {
		return "", errors.New("no target configuration provided, and didn't find exactly one install device")
	}

	// Loop through all disks, selecting the first one that isn't the source and matches the Target configuration.
	for _, device := range potentialTargets {
		if device.KName == sourceDevice {
			continue
		}

		if seedTarget == nil || strings.Contains(device.ID, seedTarget.ID) {
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

	// If ForceInstall is true, zap any existing GPT table on the target device.
	if i.config.ForceInstall {
		_, err := subprocess.RunCommandContext(ctx, "sgdisk", "-Z", targetDevice)
		if err != nil {
			return err
		}
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

	if !sourceIsReadonly {
		// Delete auto-created partitions from source device before proceeding with the install, so we can
		// re-use the installer media on other systems.
		for i := 9; i <= 11; i++ {
			_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-d", strconv.Itoa(i), sourceDevice)
			if err != nil {
				return err
			}
		}
	}

	// Number of partitions to copy.
	numPartitionsToCopy := 8
	if sourceIsReadonly {
		numPartitionsToCopy = 5
	}

	// If we're running from a CDROM, fixup the actual device we should look at for the partitions.
	actualSourceDevice := sourceDevice
	if actualSourceDevice == cdromMappedDevice {
		actualSourceDevice = "/dev/sr0"
	}

	i.tui.DisplayModal("Incus OS Install", "Cloning GPT partitions.", 0, 0)

	// Copy partition definitions.
	for idx := 1; idx <= numPartitionsToCopy; idx++ {
		err := copyPartitionDefinition(ctx, actualSourceDevice, targetDevice, idx)
		if err != nil {
			return err
		}
	}

	// If we're running from a read-only media, cheat a bit and pre-create the three additional empty
	// partitions rather than relying on systemd-repart to do so at first boot time. This is because
	// systemd-repart likes to place the small /usr-verity sig partition prior to the ESP partition.
	if sourceIsReadonly {
		_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", "6::+16KiB", "-t", "6:8385", "-c", "6:_empty", targetDevice)
		if err != nil {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", "7::+100MiB", "-t", "7:8319", "-c", "7:_empty", targetDevice)
		if err != nil {
			return err
		}

		_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", "8::+1GiB", "-t", "8:8314", "-c", "8:_empty", targetDevice)
		if err != nil {
			return err
		}
	}

	// Get partition prefixes, if needed.
	sourcePartitionPrefix := getPartitionPrefix(sourceDevice)
	targetPartitionPrefix := getPartitionPrefix(targetDevice)

	// Format the target ESP partition and manually copy any files from the source.
	// This is a speed optimization since we don't care about copying any unused data
	// from the source.
	i.tui.DisplayModal("Incus OS Install", "Copying ESP partition data.", 0, 0)

	_, err = subprocess.RunCommandContext(ctx, "mkfs.vfat", "-n", "ESP", targetDevice+targetPartitionPrefix+"1")
	if err != nil {
		return err
	}

	err = os.Mkdir("/tmp/sourceESP", 0o755)
	if err != nil {
		return err
	}

	err = os.Mkdir("/tmp/targetESP", 0o755)
	if err != nil {
		return err
	}

	err = unix.Mount(sourceDevice+sourcePartitionPrefix+"1", "/tmp/sourceESP", "vfat", 0, "ro")
	if err != nil {
		return err
	}

	err = unix.Mount(targetDevice+targetPartitionPrefix+"1", "/tmp/targetESP", "vfat", 0, "")
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "sh", "-c", "cp -ar /tmp/sourceESP/* /tmp/targetESP/")
	if err != nil {
		return err
	}

	err = unix.Unmount("/tmp/sourceESP", 0)
	if err != nil {
		return err
	}

	err = unix.Unmount("/tmp/targetESP", 0)
	if err != nil {
		return err
	}

	// Copy the partition contents. We skip the first (ESP) partition, because we've copied
	// everything in that partition above.
	for idx := 2; idx <= numPartitionsToCopy; idx++ {
		err := i.doCopy(sourceDevice, sourcePartitionPrefix, targetDevice, targetPartitionPrefix, idx, numPartitionsToCopy)
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

	// Finally, run `bootctl install`.
	err = os.MkdirAll("/boot", 0o755)
	if err != nil {
		return err
	}

	err = unix.Mount(targetDevice+targetPartitionPrefix+"1", "/boot", "vfat", 0, "")
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "bootctl", "install")
	if err != nil {
		return err
	}

	err = unix.Unmount("/boot", 0)

	return err
}

// Copy partition definitions to target device. We can't just do a `sgdisk -R target source`
// because the install media may have a different sector size than the target device (for example,
// if the installer is running from a CDROM).
func copyPartitionDefinition(ctx context.Context, src string, tgt string, partitionIndex int) error {
	// Get source partition information.
	output, err := subprocess.RunCommandContext(ctx, "sgdisk", "-i", strconv.Itoa(partitionIndex), src)
	if err != nil {
		return err
	}

	// Annoyingly, sgdisk exits with zero if given a non-existent partition.
	if strings.Contains(output, "does not exist") {
		return errors.New(output)
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

func (i *Install) doCopy(sourceDevice string, sourcePartitionPrefix string, targetDevice string, targetPartitionPrefix string, partitionIndex int, numPartitionsToCopy int) error {
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

// rebootUponDeviceRemoval waits for the given device to disappear from /dev/, and once it does
// it will reboot the system. If ForceReoot is true in the config, the system will reboot immediately.
func (i *Install) rebootUponDeviceRemoval(_ context.Context, device string) error {
	partition := fmt.Sprintf("%s%s1", device, getPartitionPrefix(device))

	// If we're running from a CDROM, adjust the device we watch for removal.
	if device == cdromMappedDevice {
		partition = "/dev/sr0"
	}

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
