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
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
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

// CheckSystemRequirements verifies that the system meets the minimum requirements for running IncusOS.
func CheckSystemRequirements(ctx context.Context, t *tui.TUI) error {
	// Check if Secure Boot is enabled.
	sbEnabled, err := secureboot.Enabled()
	if err != nil {
		return err
	}

	// Check if a TPM device, either hardware or swtpm, is present and working.
	_, tpmErr := subprocess.RunCommandContext(ctx, "tpm2_selftest")

	// Determine if there's a working physical TPM.
	workingPhysicalTPM := tpmErr == nil && !secureboot.GetSWTPMInUse()

	// If Secure Boot is disabled and there's no working physical TPM, refuse to continue.
	if !sbEnabled && !workingPhysicalTPM {
		return errors.New("cannot run if Secure Boot is disabled and no physical TPM is present")
	}

	// Get the install seed, if it exists.
	installSeed, err := seed.GetInstall()
	if err != nil && !seed.IsMissing(err) && !errors.Is(err, io.EOF) {
		return errors.New("unable to get install seed: " + err.Error())
	}

	// Validate that the install seed, if present, doesn't attempt to configure an invalid degraded security state.
	if installSeed != nil && installSeed.Security != nil {
		if installSeed.Security.MissingTPM && installSeed.Security.MissingSecureBoot {
			return errors.New("install seed cannot enable both Secure Boot and TPM degraded security options")
		}

		// Return an error if there's a physical TPM but the install seed wants to configure swtpm.
		if workingPhysicalTPM && installSeed.Security.MissingTPM {
			return errors.New("a physical TPM was found, but install seed wants to configure a swtpm-backed TPM")
		}

		// Return an error if Secure Boot is enabled but the install seed expects it to be disabled.
		if sbEnabled && installSeed.Security.MissingSecureBoot {
			return errors.New("Secure Boot is enabled, but install seed expects it to be disabled") //nolint:staticcheck
		}
	}

	// If Secure Boot is enabled, but there's no working TPM, attempt to initialize swtpm for use on next boot.
	if sbEnabled && tpmErr != nil {
		// Return an error if there's no working TPM and the install seed doesn't allow using swtpm.
		if installSeed != nil && (installSeed.Security == nil || !installSeed.Security.MissingTPM) {
			return errors.New("no working TPM found, but install seed doesn't allow for use of swtpm")
		}

		err := configureSWTPM(ctx, t, installSeed != nil)
		if err != nil {
			return err
		}
	}

	// If Secure Boot is disabled and there's a working physical TPM, allow running.
	if !sbEnabled && workingPhysicalTPM {
		// Return an error if Secure Boot is disabled and the install seed doesn't allow this.
		if installSeed != nil && (installSeed.Security == nil || !installSeed.Security.MissingSecureBoot) {
			return errors.New("Secure Boot is disabled, but install seed doesn't allow this") //nolint:staticcheck
		}

		// Only display warning during install or first boot.
		_, err := os.Stat("/boot/sb-disabled")
		if err != nil && errors.Is(err, os.ErrNotExist) {
			displayDegradedSecurityWarning(t, "Disabling Secure Boot")

			// Create the flag file here if live boot, otherwise it will be created on the new
			// ESP partition at the conclusion of the install.
			if installSeed == nil {
				fd, err := os.Create("/boot/sb-disabled")
				if err != nil {
					return err
				}

				_ = fd.Close()
			}
		}
	}

	// Get the source device that IncusOS is running from.
	sourceDevice, sourceIsReadonly, sourceDeviceSize, err := getSourceDevice(ctx)
	if err != nil {
		return errors.New("unable to determine source device: " + err.Error())
	}

	// If we aren't going to perform an install but systemd-repart failed or we're running from a CDROM,
	// display an appropriate error message to the user.
	if !ShouldPerformInstall() && (systemd.IsFailed(ctx, "systemd-repart") || runningFromCDROM()) {
		if sourceIsReadonly {
			return fmt.Errorf("unable to begin install from read-only device '%s' without seed configuration", sourceDevice)
		}

		if sourceDeviceSize < 50*1024*1024*1024 {
			return fmt.Errorf("source device '%s' is too small (%0.2fGiB), must be at least 50GiB", sourceDevice, float64(sourceDeviceSize)/(1024.0*1024.0*1024.0))
		}

		return errors.New("no install seed provided, and failed to run systemd-repart for live system")
	}

	// Perform install-specific checks.
	if installSeed != nil { //nolint:nestif
		targets, err := getAllTargets(ctx, sourceDevice)
		if err != nil {
			return errors.New("unable to get list of potential target devices: " + err.Error())
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
			if !strings.HasPrefix(seedPartition, sourceDevice) {
				return errors.New("install media detected, but the system is already installed; please remove USB/CDROM and reboot the system")
			}
		}

		targetDevice, targetDeviceSize, err := getTargetDevice(targets, installSeed.Target)
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

		// If an applications seed is present, ensure at least one application is defined.
		apps, _ := seed.GetApplications(ctx)
		if apps != nil {
			if len(apps.Applications) == 0 {
				return errors.New("at least one application must be defined in the provided applications seed")
			}
		}
	}

	return nil
}

// ShouldPerformInstall checks for the presence of an install.{json,yaml} file in the
// seed partition to indicate if we should attempt to install incus-osd to a local disk.
func ShouldPerformInstall() bool {
	_, err := seed.GetInstall()

	return err == nil
}

// NewInstall returns a new Install object with its configuration, if any, populated from the seed partition.
func NewInstall(t *tui.TUI) (*Install, error) {
	ret := &Install{
		tui: t,
	}

	var err error

	ret.config, err = seed.GetInstall()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	return ret, nil
}

// DoInstall performs the necessary steps for installing incus-osd to a local disk.
func (i *Install) DoInstall(ctx context.Context, osName string) error {
	modal := i.tui.AddModal(osName+" Install", "install")
	slog.InfoContext(ctx, "Starting install of "+osName+" to local disk")
	modal.Update("Starting install of " + osName + " to local disk.")

	sourceDevice, sourceIsReadonly, _, err := getSourceDevice(ctx)
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
func getSourceDevice(ctx context.Context) (string, bool, int, error) { //nolint:revive
	// Check if we're running from a CDROM.
	if runningFromCDROM() {
		return cdromMappedDevice, true, -1, nil
	}

	// If boot.mount has failed, we're running from a read-only USB stick.
	// (fsck.fat fails on the read-only ESP partition.) Can't use systemd.IsFailed(),
	// since systemd doesn't actually report the mount unit as failed, so we
	// need to check its output.
	output, err := subprocess.RunCommandContext(ctx, "journalctl", "-b", "-u", "boot.mount")
	if err != nil {
		return "", false, -1, err
	}

	isReadonlyInstallFS := strings.Contains(output, "Dependency failed for boot.mount - EFI System Partition Automount.")

	underlyingDevice, err := storage.GetUnderlyingDevice()
	if err != nil {
		return "", isReadonlyInstallFS, -1, err
	}

	// Get the device's size.
	lsblkOutput := storage.LsblkOutput{}

	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-iJnpbs", "-o", "KNAME,ID_LINK,SIZE", underlyingDevice)
	if err != nil {
		return "", isReadonlyInstallFS, -1, err
	}

	err = json.Unmarshal([]byte(output), &lsblkOutput)
	if err != nil {
		return "", isReadonlyInstallFS, -1, err
	}

	return underlyingDevice, isReadonlyInstallFS, lsblkOutput.BlockDevices[0].Size, nil
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

	// Get MMC drives third.
	mmcTargets := storage.LsblkOutput{}

	// MMC block devices have major number 179 (https://www.kernel.org/doc/Documentation/admin-guide/devices.txt)
	output, err = subprocess.RunCommandContext(ctx, "lsblk", "-I", "179", "-iJnpb", "-o", "KNAME,ID_LINK,SIZE")
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	err = json.Unmarshal([]byte(output), &mmcTargets)
	if err != nil {
		return []storage.BlockDevices{}, err
	}

	ret = append(ret, mmcTargets.BlockDevices...)

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

		if entry.ID == "" {
			// Skip devices that don't have a link ID, such as mmcblk0boot0.
			continue
		}

		if strings.HasPrefix(entry.ID, "usb-Linux_Virtual_") {
			// Virtual BMC devices on DELL servers.
			continue
		}

		if strings.HasPrefix(entry.ID, "usb-Cisco_") {
			// Virtual BMC devices on Cisco servers.
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
	sourcePartitionPrefix := GetPartitionPrefix(sourceDevice)
	targetPartitionPrefix := GetPartitionPrefix(targetDevice)

	// Format the target ESP partition and manually copy any files from the source.
	// This is a speed optimization since we don't care about copying any unused data
	// from the source.
	modal.Update("Copying ESP partition data.")

	_, err = subprocess.RunCommandContext(ctx, "mkfs.vfat", "-n", "ESP", targetDevice+targetPartitionPrefix+"1")
	if err != nil {
		return err
	}

	err = os.MkdirAll("/tmp/sourceESP", 0o755)
	if err != nil {
		return err
	}

	err = os.MkdirAll("/tmp/targetESP", 0o755)
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

	// Remove the install seed from the target device, and copy any external user-provided seeds.
	err = seed.CleanupPostInstall(ctx, fmt.Sprintf("%s%s2", targetDevice, targetPartitionPrefix))
	if err != nil {
		return err
	}

	// Mount /boot/ for final install steps.
	err = os.MkdirAll("/boot", 0o755)
	if err != nil {
		return err
	}

	err = unix.Mount(targetDevice+targetPartitionPrefix+"1", "/boot", "vfat", 0, "umask=0077")
	if err != nil {
		return err
	}

	// If swtpm state was configured, move it to the ESP partition.
	_, err = os.Stat("/tmp/swtpm/")
	if err == nil {
		_, err = subprocess.RunCommandContext(ctx, "sh", "-c", "mv /tmp/swtpm/ /boot/")
		if err != nil {
			return err
		}
	}

	// If Secure Boot is disabled, create the flag file on the ESP partition.
	sbEnabled, err := secureboot.Enabled()
	if err != nil {
		return err
	}

	if !sbEnabled {
		fd, err := os.Create("/boot/sb-disabled")
		if err != nil {
			return err
		}

		_ = fd.Close()
	}

	// Finally, run `bootctl install`.
	_, err = subprocess.RunCommandContext(ctx, "bootctl", "--graceful", "install")
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
	partition := fmt.Sprintf("%s%s1", device, GetPartitionPrefix(device))

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

	// Wait 5s to better handle virtual machines with virtual media.
	// This is to avoid them dealing with both a media removal and reboot concurently.
	time.Sleep(5 * time.Second)

	return os.WriteFile("/proc/sysrq-trigger", []byte("b"), 0o600)
}

// GetPartitionPrefix returns the necessary partition prefix, if any, for a give device.
// nvme and mmc devices have partitions named "pN", while traditional disk partitions are just "N".
func GetPartitionPrefix(device string) string {
	cdromMatched, _ := regexp.MatchString(`/mapper/sr\d+`, device)

	if strings.Contains(device, "/nvme") || strings.Contains(device, "/mmcblk") || cdromMatched {
		return "p"
	}

	return ""
}

// configureSWTPM will configure the swtpm-based TPM after displaying a warning modal. If
// IncusOS is running live from the USB drive, configure swtpm and immediately reboot so the
// live system can have a proper TPM.
func configureSWTPM(ctx context.Context, t *tui.TUI, isInstall bool) error {
	displayDegradedSecurityWarning(t, "A software-backed TPM")

	if isInstall {
		// At the conclusion of the install, the swtpm state will be copied to the new ESP partition.
		return initializeSWTPM(ctx, "/tmp/swtpm/")
	}

	_, err := os.Stat("/boot/swtpm/")
	if err == nil {
		return errors.New("swtpm was already configured")
	}

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("unable to get swtpm state directory: " + err.Error())
	}

	err = initializeSWTPM(ctx, "/boot/swtpm/")
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Configuring swtpm-backed TPM on first boot, restarting in five seconds")

	time.Sleep(5 * time.Second)

	_ = systemd.SystemReboot(ctx)

	// Prevent further system start up in the half second or so before things reboot.
	time.Sleep(60 * time.Second)

	return nil
}

// initializeSWTPM initializes the swtpm state in the given root directory.
func initializeSWTPM(ctx context.Context, swtpmRoot string) error {
	// Create the swtpm state directory.
	err := os.MkdirAll(swtpmRoot, 0o700)
	if err != nil {
		return err
	}

	// Create swtpm_setup config files.
	err = os.WriteFile("/etc/swtpm_setup.conf", []byte("create_certs_tool = swtpm_localca"), 0o644)
	if err != nil {
		return err
	}

	err = os.WriteFile("/etc/swtpm-localca.options", []byte("--platform-manufacturer IncusOS\n--platform-version 1.0\n--platform-model QEMU"), 0o644)
	if err != nil {
		return err
	}

	err = os.WriteFile("/etc/swtpm-localca.conf", []byte("statedir = /tmp/swtpm_localca/\nsigningkey = /tmp/swtpm_localca/signkey.pem\nissuercert = /tmp/swtpm_localca/issuercert.pem\ncertserial = /tmp/swtpm_localca/certserial"), 0o644)
	if err != nil {
		return err
	}

	// Initialize the TPM.
	_, err = subprocess.RunCommandContext(ctx, "swtpm_setup", "--tpm2", "--tpmstate", swtpmRoot, "--create-ek-cert", "--create-platform-cert", "--lock-nvram")

	return err
}

func displayDegradedSecurityWarning(t *tui.TUI, msg string) {
	modal := t.AddModal("Degraded security warning", "degraded-security-warning")

	for i := range 30 {
		secondsString := "seconds"
		if 30-i == 1 {
			secondsString = "second"
		}

		modal.Update(fmt.Sprintf("[red]WARNING:[white] %s will result in a degraded security state.\nContinuing in %d %s...", msg, 30-i, secondsString))

		time.Sleep(1 * time.Second)
	}

	modal.Done()
}
