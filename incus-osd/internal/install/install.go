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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/tui"
)

// Install holds information necessary to perform an installation.
type Install struct {
	config *apiseed.Install
	tui    *tui.TUI
}

var cdromDevice = "/dev/sr0"

var cdromMappedDevice = "/dev/mapper/sr0"

var cdromRegex = regexp.MustCompile(`^/dev/sr(\d+)`)

// CheckSystemRequirements verifies that the system meets the minimum requirements for running Incus OS.
func CheckSystemRequirements(ctx context.Context) error {
	// Check if Secure Boot is enabled.
	output, err := subprocess.RunCommandContext(ctx, "bootctl", "status")
	if err != nil {
		return err
	} else if !strings.Contains(output, "Secure Boot: enabled") {
		return errors.New("Secure Boot is not enabled") //nolint:staticcheck
	}

	// Check if a TPM device is present and working.
	_, err = subprocess.RunCommandContext(ctx, "tpm2_selftest")
	if err != nil {
		return errors.New("no working TPM device found")
	}

	// Check if systemd-repart has failed (we're either running from a read-only or a small USB
	// stick), or we're running from a CDROM, which normally indicates we're about to start an
	// install but there's no install seed present.
	if (systemd.IsFailed(ctx, "systemd-repart") || runningFromCDROM()) && !ShouldPerformInstall() {
		return errors.New("unable to begin install without seed configuration")
	}

	// Perform install-specific checks.
	if ShouldPerformInstall() { //nolint:nestif
		// Check that we have either been told what target device to use, or that we can automatically figure it out.
		source, _, err := getSourceDevice(ctx)
		if err != nil {
			return errors.New("unable to determine source device: " + err.Error())
		}

		targets, err := getAllTargets(ctx, source)
		if err != nil {
			return errors.New("unable to get list of potential target devices: " + err.Error())
		}

		config, err := seed.GetInstall(seed.GetSeedPath())
		if err != nil && !errors.Is(err, io.EOF) {
			return errors.New("unable to get seed config: " + err.Error())
		}

		// Sanity check: if we're not running from a CDROM, ensure that the default install media seed partition
		// exists on the source device. If not, there are at least two IncusOS drives present, the installed system
		// and an install media. This will result in a weird environment, so raise an error telling the user to
		// remove the install device.
		//
		// This won't catch the case when an external user-provided seed partition is present, and we rely on the
		// user properly removing their seed device post-install.
		if !runningFromCDROM() {
			seedLink, err := os.Readlink("/dev/disk/by-partlabel/seed-data")
			if err != nil {
				return err
			}

			seedPartition := filepath.Join("/dev/disk/by-partlabel", seedLink)
			if !strings.HasPrefix(seedPartition, source) {
				return errors.New("install media detected, but the system is already installed; please remove USB/CDROM and reboot the system")
			}
		}

		targetDevice, targetDeviceSize, err := getTargetDevice(targets, config.Target)
		if err != nil {
			devices := []string{}
			for _, t := range targets {
				devices = append(devices, t.ID)
			}

			return errors.New(err.Error() + " (detected devices: " + strings.Join(devices, ", ") + ")")
		}

		// Verify the target device is at least 50GiB.
		if targetDeviceSize < 50*1024*1024*1024 {
			return fmt.Errorf("target device '%s' is too small (%0.2fGiB), must be at least 50GiB", targetDevice, float64(targetDeviceSize)/(1024.0*1024.0*1024.0))
		}
	}

	return nil
}

// ShouldPerformInstall checks for the presence of an install.{json,yaml} file in the
// seed partition to indicate if we should attempt to install incus-osd to a local disk.
func ShouldPerformInstall() bool {
	_, err := seed.GetInstall(seed.GetSeedPath())

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

	ret.config, err = seed.GetInstall(seed.GetSeedPath())
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	return ret, nil
}

// DoInstall performs the necessary steps for installing incus-osd to a local disk.
func (i *Install) DoInstall(ctx context.Context, osName string) error {
	modal := i.tui.AddModal(osName + " Install")
	slog.InfoContext(ctx, "Starting install of "+osName+" to local disk")
	modal.Update("Starting install of " + osName + " to local disk.")

	sourceDevice, sourceIsReadonly, err := getSourceDevice(ctx)
	if err != nil {
		modal.Update("[red]Error: " + err.Error())

		return err
	}

	targets, err := getAllTargets(ctx, sourceDevice)
	if err != nil {
		modal.Update("[red]Error: " + err.Error())

		return err
	}

	targetDevice, _, err := getTargetDevice(targets, i.config.Target)
	if err != nil {
		modal.Update("[red]Error: " + err.Error())

		return err
	}

	slog.InfoContext(ctx, "Installing "+osName, "source", sourceDevice, "target", targetDevice)
	modal.Update(fmt.Sprintf("Installing "+osName+" from %s to %s.", sourceDevice, targetDevice))

	err = i.performInstall(ctx, modal, sourceDevice, targetDevice, sourceIsReadonly)
	if err != nil {
		modal.Update("[red]Error: " + err.Error())

		return err
	}

	slog.InfoContext(ctx, osName+" was successfully installed")
	slog.InfoContext(ctx, "Please remove the install media to complete the installation")
	modal.Update(osName + " was successfully installed.\nPlease remove the install media to complete the installation.")

	return i.rebootUponDeviceRemoval(ctx, sourceDevice)
}

// runningFromCDROM returns true we're running from a CDROM, which should only happen during an install.
func runningFromCDROM() bool {
	underlyingDevice, err := storage.GetUnderlyingDevice()
	if err != nil {
		return false
	}

	if !cdromRegex.MatchString(underlyingDevice) {
		// Not running from a CDROM.
		return false
	}

	// Most of the time we'll be running from /dev/sr0; if not, update variables as needed.
	if underlyingDevice != cdromDevice {
		cdromDevice = underlyingDevice
		cdromMappedDevice = "/dev/mapper/sr" + cdromRegex.FindStringSubmatch(underlyingDevice)[1]
	}

	return true
}

// getSourceDevice determines the underlying device incus-osd is running on and if it is read-only.
func getSourceDevice(ctx context.Context) (string, bool, error) {
	// Check if we're running from a CDROM.
	if runningFromCDROM() {
		return cdromMappedDevice, true, nil
	}

	// If boot.mount has failed, we're running from a read-only USB stick.
	// (fsck.fat fails on the read-only ESP partition.) Can't use systemd.IsFailed(),
	// since systemd doesn't actually report the mount unit as failed, so we
	// need to check its output.
	output, err := subprocess.RunCommandContext(ctx, "journalctl", "-b", "-u", "boot.mount")
	if err != nil {
		return "", false, err
	}

	isReadonlyInstallFS := strings.Contains(output, "Dependency failed for boot.mount - EFI System Partition Automount.")

	underlyingDevice, err := storage.GetUnderlyingDevice()
	if err != nil {
		return "", isReadonlyInstallFS, err
	}

	return underlyingDevice, isReadonlyInstallFS, nil
}

// getAllTargets returns a list of all potential install target devices.
func getAllTargets(ctx context.Context, sourceDevice string) ([]storage.BlockDevices, error) {
	ret := []storage.BlockDevices{}

	// Get NVME drives first.
	nvmeTargets := storage.LsblkOutput{}

	output, err := subprocess.RunCommandContext(ctx, "lsblk", "-N", "-iJnpb", "-e", "1,2", "-o", "KNAME,ID_LINK,SIZE")
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	err = json.Unmarshal([]byte(output), &nvmeTargets)
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	ret = append(ret, nvmeTargets.BlockDevices...)

	// Get SCSI drives second.
	scsiTargets := storage.LsblkOutput{}

	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-S", "-iJnpb", "-e", "1,2", "-o", "KNAME,ID_LINK,SIZE")
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	err = json.Unmarshal([]byte(output), &scsiTargets)
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	ret = append(ret, scsiTargets.BlockDevices...)

	// Get virtual drives last.
	virtualTargets := storage.LsblkOutput{}

	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-v", "-iJnpb", "-e", "1,2", "-o", "KNAME,ID_LINK,SIZE")
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	err = json.Unmarshal([]byte(output), &virtualTargets)
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	ret = append(ret, virtualTargets.BlockDevices...)

	// Filter out devices that are known to not be valid targets.
	filtered := make([]storage.BlockDevices, 0, len(ret))
	for _, entry := range ret {
		if entry.KName == sourceDevice {
			continue
		}

		if strings.HasPrefix(entry.ID, "usb-Linux_Virtual_") {
			// Virtual BMC devices on DELL servers.
			continue
		}

		if cdromRegex.MatchString(entry.KName) {
			// Ignore all CDROM devices.
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered, nil
}

// getTargetDevice determines the underlying device and its size in bytes to install incus-osd on.
func getTargetDevice(potentialTargets []storage.BlockDevices, seedTarget *apiseed.InstallTarget) (string, int, error) {
	// Ensure we found at least one potential install device. If no Target configuration was found,
	// only proceed if exactly one device was found.
	if len(potentialTargets) == 0 {
		return "", -1, errors.New("no potential install devices found")
	} else if seedTarget == nil && len(potentialTargets) != 1 {
		return "", -1, errors.New("no target configuration provided, and didn't find exactly one install device")
	}

	// Loop through all disks, selecting the first one that matches the Target configuration.
	for _, device := range potentialTargets {
		// First, check for a simple substring match.
		if seedTarget == nil || strings.Contains(device.ID, seedTarget.ID) {
			return device.KName, device.Size, nil
		}

		// Second, check if the specified target ID and current device are both symlinks to the same underlying device.
		seedDeviceLink, err := os.Readlink(filepath.Join("/dev/disk/by-id", seedTarget.ID))
		if err == nil {
			potentialDeviceLink, err := os.Readlink(filepath.Join("/dev/disk/by-id", device.ID))
			if err == nil && seedDeviceLink == potentialDeviceLink {
				return device.KName, device.Size, nil
			}
		}
	}

	if seedTarget == nil {
		return "", -1, errors.New("unable to determine target device")
	}

	return "", -1, errors.New("no target device matched '" + seedTarget.ID + "'")
}

// performInstall performs the steps to install incus-osd from the given target to the source device.
func (i *Install) performInstall(ctx context.Context, modal *tui.Modal, sourceDevice string, targetDevice string, sourceIsReadonly bool) error {
	// Get architecture name.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return err
	}

	if !slices.Contains([]string{"x86_64", "aarch64"}, archName) {
		return fmt.Errorf("unsupported architecture %q", archName)
	}

	// Check if the target device already has a partition table.
	output, err := subprocess.RunCommandContext(ctx, "sgdisk", "-v", targetDevice)
	if err != nil {
		// If the device has no main partition table, but does have a backup, assume it's been
		// partially wiped with something like `dd if=/dev/zero of=/dev/sda ...` and proceed with install.
		if !strings.Contains(err.Error(), "Caution: invalid main GPT header, but valid backup; regenerating main header") {
			return err
		}

		// Set ForceInstall to true in this case since the install should continue.
		i.config.ForceInstall = true
	}

	if !strings.Contains(output, "Creating new GPT entries in memory") && !i.config.ForceInstall {
		return fmt.Errorf("a partition table already exists on device '%s', and `ForceInstall` from install configuration isn't true", targetDevice)
	}

	// At this point, the target device either has no GPT table, or we will be force-installing over any existing data.

	// Zap any existing GPT table on the target device.
	if i.config.ForceInstall {
		// Don't check return status, since sgdisk always returns an error if there's a mismatch
		// between the main and backup GPT tables.
		_, _ = subprocess.RunCommandContext(ctx, "sgdisk", "-Z", targetDevice)
	}

	// Before starting the install, run blkdiscard to fully wipe the target device. blkdiscard may
	// not work for all devices, so don't check its return status.
	_, _ = subprocess.RunCommandContext(ctx, "blkdiscard", "-f", targetDevice)

	// Turn off swap and unmount /boot.
	_, err = subprocess.RunCommandContext(ctx, "swapoff", "-a")
	if err != nil {
		return err
	}

	err = unix.Unmount("/boot/", 0)
	if err != nil {
		// /boot/ won't be mounted when installer is running from read-only media.
		if !sourceIsReadonly {
			return err
		}
	}

	// If we're running from a CDROM, fixup the actual device we should look at for the partitions.
	actualSourceDevice := sourceDevice
	if actualSourceDevice == cdromMappedDevice {
		actualSourceDevice = cdromDevice
	}

	output, err = subprocess.RunCommandContext(ctx, "sgdisk", "-i", "9", actualSourceDevice)
	if err != nil {
		return err
	}

	if !strings.Contains(output, "Partition #9 does not exist.") {
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

	output, err = subprocess.RunCommandContext(ctx, "sgdisk", "-i", "8", actualSourceDevice)
	if err != nil {
		return err
	}

	if strings.Contains(output, "Partition #8 does not exist.") {
		numPartitionsToCopy = 5
	}

	modal.Update("Cloning GPT partitions.")

	// Copy partition definitions.
	for idx := 1; idx <= numPartitionsToCopy; idx++ {
		err := copyPartitionDefinition(ctx, actualSourceDevice, targetDevice, idx)
		if err != nil {
			return err
		}
	}

	// If we're running from media with only the first five partitions, cheat a bit and pre-create
	// the other three additional empty partitions rather than relying on systemd-repart to do so
	// at first boot time. This is because systemd-repart likes to place the small /usr-verity sig
	// partition prior to the ESP partition.
	if numPartitionsToCopy == 5 {
		switch archName {
		case "aarch64":
			_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", "6::+16KiB", "-t", "6:8375", "-c", "6:_empty", targetDevice)
			if err != nil {
				return err
			}

			_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", "7::+100MiB", "-t", "7:831B", "-c", "7:_empty", targetDevice)
			if err != nil {
				return err
			}

			_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", "8::+1GiB", "-t", "8:8316", "-c", "8:_empty", targetDevice)
			if err != nil {
				return err
			}

		case "x86_64":
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

		default:
		}
	}

	// Get partition prefixes, if needed.
	sourcePartitionPrefix := getPartitionPrefix(sourceDevice)
	targetPartitionPrefix := getPartitionPrefix(targetDevice)

	// Format the target ESP partition and manually copy any files from the source.
	// This is a speed optimization since we don't care about copying any unused data
	// from the source.
	modal.Update("Copying ESP partition data.")

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
		err := doCopy(ctx, modal, sourceDevice, sourcePartitionPrefix, targetDevice, targetPartitionPrefix, idx, numPartitionsToCopy)
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

	var partitionHexCode string

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

	case "Linux ARM64 /usr verity signature":
		partitionHexCode = "8375"
	case "Linux ARM64 /usr verity":
		partitionHexCode = "831b"
	case "Linux ARM64 /usr":
		partitionHexCode = "8316"
	default:
		return fmt.Errorf("unrecognized partition type '%s'", partitionType)
	}

	// Create the partition on the target device.
	_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-n", strconv.Itoa(partitionIndex)+"::+"+partitionSize, "-u", strconv.Itoa(partitionIndex)+":"+partitionGUID, "-t", strconv.Itoa(partitionIndex)+":"+partitionHexCode, "-c", strconv.Itoa(partitionIndex)+":"+partitionName, tgt)

	return err
}

func doCopy(ctx context.Context, modal *tui.Modal, sourceDevice string, sourcePartitionPrefix string, targetDevice string, targetPartitionPrefix string, partitionIndex int, numPartitionsToCopy int) error {
	sourcePartition, err := os.OpenFile(fmt.Sprintf("%s%s%d", sourceDevice, sourcePartitionPrefix, partitionIndex), os.O_RDONLY, 0o0600)
	if err != nil {
		return err
	}
	defer sourcePartition.Close()

	var partitionSize int64

	// Optimize copying of the /usr erofs partition by only copying the actual data.
	output, err := subprocess.RunCommandContext(ctx, "dump.erofs", fmt.Sprintf("%s%s%d", sourceDevice, sourcePartitionPrefix, partitionIndex))
	if err == nil {
		blocksizeRegex := regexp.MustCompile(`Filesystem blocksize:                         (.+)`)
		blocksRegex := regexp.MustCompile(`Filesystem blocks:                            (.+)`)

		blocksize, err := strconv.Atoi(blocksizeRegex.FindStringSubmatch(output)[1])
		if err != nil {
			return err
		}

		blocks, err := strconv.Atoi(blocksRegex.FindStringSubmatch(output)[1])
		if err != nil {
			return err
		}

		partitionSize = int64(blocksize * blocks)
	} else {
		// Not an erofs image, so fallback to whole partition.
		partitionSize, err = sourcePartition.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}

		_, err = sourcePartition.Seek(0, 0)
		if err != nil {
			return err
		}
	}

	targetPartition, err := os.OpenFile(fmt.Sprintf("%s%s%d", targetDevice, targetPartitionPrefix, partitionIndex), os.O_WRONLY, 0o0600)
	if err != nil {
		return err
	}
	defer targetPartition.Close()

	modal.Update(fmt.Sprintf("Copying partition %d of %d (%.2fMiB).", partitionIndex, numPartitionsToCopy, float64(partitionSize)/1024.0/1024.0))

	// Copy data in 4MiB chunks.
	blockSize := int64(4 * 1024 * 1024)
	count := int64(0)

	for {
		_, err := io.CopyN(targetPartition, sourcePartition, blockSize)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		// Update progress every 24MiB.
		if count%6 == 0 {
			modal.UpdateProgress(float64(count*blockSize) / float64(partitionSize))
		}

		count++

		// Break out of copy loop early, if possible.
		if count*blockSize > partitionSize {
			break
		}
	}

	// Hide the progress bar.
	modal.UpdateProgress(0.0)

	return nil
}

// rebootUponDeviceRemoval waits for the given device to disappear from /dev/, and once it does
// it will reboot the system. If ForceReoot is true in the config, the system will reboot immediately.
func (i *Install) rebootUponDeviceRemoval(_ context.Context, device string) error {
	partition := fmt.Sprintf("%s%s1", device, getPartitionPrefix(device))

	// If we're running from a CDROM, adjust the device we watch for removal.
	if device == cdromMappedDevice {
		partition = cdromDevice
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
	cdromMatched, _ := regexp.MatchString(`/mapper/sr\d+`, device)

	if strings.Contains(device, "/nvme") || cdromMatched {
		return "p"
	}

	return ""
}
