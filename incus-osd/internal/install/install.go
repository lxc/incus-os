package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
)

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

// GetSourceDevice determines the underlying device incus-osd is running on.
func GetSourceDevice(ctx context.Context) (string, error) {
	output, err := subprocess.RunCommandContext(ctx, "sh", "-c", `MAJOR_MINOR=$(dmsetup deps usr | sed -E 's/^.+: \(([0-9]+), ([0-9]+)\).+$/\1:\2/'); PARTITION=$(udevadm info -rq name /sys/dev/block/${MAJOR_MINOR}); lsblk --list --noheadings --paths --output PKNAME ${PARTITION} | head -n 1`)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(output, "\n"), nil
}

// GetTargetDevice determines the underlying device to install incus-osd on.
func GetTargetDevice(ctx context.Context, sourceDevice string) (string, error) {
	config, err := seed.GetInstallConfig(seed.SeedPartitionPath)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

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

	// If no target substring is provided, we can only proceed if exactly two disks were found:
	// the install media, and a single target disk.
	if config.TargetDiskSubstring == "" && len(potentialTargets) != 2 {
		if len(potentialTargets) < 2 {
			return "", errors.New("unable to find a target device")
		}

		return "", errors.New("more than one potential target device found, and no substring for matching was configured")
	}

	// Now, loop through all disks, selecting the first one that isn't the source and matches
	// the configured substring.
	for _, device := range potentialTargets {
		if device.KName == sourceDevice {
			continue
		}

		if strings.Contains(device.ID, config.TargetDiskSubstring) {
			return device.KName, nil
		}
	}

	return "", errors.New("unable to determine target device")
}

// DoInstall performs the steps to install incus-osd from the given target to the source device.
func DoInstall(ctx context.Context, sourceDevice string, targetDevice string) error {
	config, err := seed.GetInstallConfig(seed.SeedPartitionPath)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	// Verify the target device doesn't already have a partition table, or that `ForceInstall` is set to true.
	output, err := subprocess.RunCommandContext(ctx, "sgdisk", "-v", targetDevice)
	if err != nil {
		return err
	}

	if !strings.Contains(output, "Creating new GPT entries in memory") && !config.ForceInstall {
		return fmt.Errorf("a partition table already exists on device '%s', and `force_install` from install configuration isn't true", targetDevice)
	}

	// Turn off swap and unmount /boot.
	_, err = subprocess.RunCommandContext(ctx, "swapoff", "-a")
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "umount", "/boot/")
	if err != nil {
		return err
	}

	// Delete auto-created partitions from source device before cloning its GPT table.
	for i := 9; i <= 11; i++ {
		_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-d", strconv.Itoa(i), sourceDevice)
		if err != nil {
			return err
		}
	}

	// Clone the GPT partition table to the target device.
	_, err = subprocess.RunCommandContext(ctx, "sgdisk", "-R", targetDevice, sourceDevice)
	if err != nil {
		return err
	}

	// Get partition prefixes, if needed.
	sourcePartitionPrefix := getPartitionPrefix(sourceDevice)
	targetPartitionPrefix := getPartitionPrefix(targetDevice)

	// Copy the partition contents.
	for i := 1; i <= 8; i++ {
		_, err = subprocess.RunCommandContext(ctx, "dd", fmt.Sprintf("if=%s%s%d", sourceDevice, sourcePartitionPrefix, i), fmt.Sprintf("of=%s%s%d", targetDevice, targetPartitionPrefix, i))
		if err != nil {
			return err
		}
	}

	// Remove the install configuration file, if present, from the target seed partition.
	targetSeedPartition := fmt.Sprintf("%s%s2", targetDevice, targetPartitionPrefix)
	for _, filename := range []string{"install.json", "install.yaml"} {
		_, err = subprocess.RunCommandContext(ctx, "sh", "-c", fmt.Sprintf(`if tar -tf %s %s; then tar -f %s --delete %s; fi`, targetSeedPartition, filename, targetSeedPartition, filename))
		if err != nil {
			return err
		}
	}

	return nil
}

// RebootUponDeviceRemoval waits for the given device to disappear from /dev/, and once it does
// it will reboot the system.
func RebootUponDeviceRemoval(ctx context.Context, device string) error {
	partition := fmt.Sprintf("%s%s1", device, getPartitionPrefix(device))

	// Wait for the partition to disappear.
	for {
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
	_, err := subprocess.RunCommandContext(ctx, "sync")
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "sh", "-c", "echo b > /proc/sysrq-trigger")
	if err != nil {
		return err
	}

	return nil
}

// getPartitionPrefix returns the necessary partition prefix, if any, for a give device.
// nvme devices have partitions named "pN", while traditional disk partitions are just "N".
func getPartitionPrefix(device string) string {
	if strings.Contains(device, "/nvme") {
		return "p"
	}

	return ""
}
