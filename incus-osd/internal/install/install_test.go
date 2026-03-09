package install

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api/seed"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
)

func TestGetTargetDeviceNoSeed(t *testing.T) {
	t.Parallel()

	devs := make([]storage.BlockDevices, 0, 2)

	// Test no targets provided.
	_, _, err := getTargetDevice(devs, nil)
	require.EqualError(t, err, "no potential install devices found")

	devs = append(devs, storage.BlockDevices{
		KName:      "/dev/sda",
		ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root",
		Size:       50,
		Subsystems: "block:scsi:virtio:pci",
		RM:         false,
	})

	// Test single target, no seed.
	target, size, err := getTargetDevice(devs, nil)
	require.NoError(t, err)
	require.Equal(t, "/dev/sda", target)
	require.Equal(t, 50, size)

	devs = append(devs, storage.BlockDevices{
		KName:      "/dev/sdb",
		ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1",
		Size:       51,
		Subsystems: "block:scsi:virtio:pci",
		RM:         false,
	})

	// Test two targets, no seed.
	_, _, err = getTargetDevice(devs, nil)
	require.EqualError(t, err, "no target install seed provided, and didn't find exactly one install device")
}

func TestGetTargetDeviceWithSeed(t *testing.T) {
	t.Parallel()

	tgt := &seed.InstallTarget{}

	devs := []storage.BlockDevices{
		{
			KName:      "/dev/sda",
			ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root",
			Size:       50,
			Subsystems: "block:scsi:virtio:pci",
			RM:         false,
		},
		{
			KName:      "/dev/sdb",
			ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1",
			Size:       51,
			Subsystems: "block:scsi:virtio:pci",
			RM:         false,
		},
		{
			KName:      "/dev/sdc",
			ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2",
			Size:       52,
			Subsystems: "block:scsi:usb:pci",
			RM:         false,
		},
		{
			KName:      "/dev/sdd",
			ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3",
			Size:       53,
			Subsystems: "block:scsi:usb:pci",
			RM:         false,
		},
		{
			KName:      "/dev/nvme0n1",
			ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4",
			Size:       60,
			Subsystems: "block:nvme:pci",
			RM:         false,
		},
		{
			KName:      "/dev/nvme1n1",
			ID:         "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5",
			Size:       61,
			Subsystems: "block:nvme:pci",
			RM:         false,
		},
	}

	// Test empty seed.
	_, _, err := getTargetDevice(devs, tgt)
	require.EqualError(t, err, "more than one target device matched provided install seed selectors")

	// Test no selectors match.
	tgt.ID = "foobar"
	_, _, err = getTargetDevice(devs, tgt)
	require.EqualError(t, err, "no target device matched provided install seed selectors")

	// Test bad sort order.
	tgt.ID = ""
	tgt.SortOrder = "foobar"
	_, _, err = getTargetDevice(devs, tgt)
	require.EqualError(t, err, "unsupported sort order 'foobar'")

	// Test bad sort order.
	tgt.ID = ""
	tgt.SortOrder = "foobar"
	_, _, err = getTargetDevice(devs, tgt)
	require.EqualError(t, err, "unsupported sort order 'foobar'")

	// Test bad max size.
	tgt.SortOrder = ""
	tgt.MaxSize = "foobar"
	_, _, err = getTargetDevice(devs, tgt)
	require.EqualError(t, err, "Invalid value: foobar")

	// Test bad min size.
	tgt.MaxSize = ""
	tgt.MinSize = "bizbaz"
	_, _, err = getTargetDevice(devs, tgt)
	require.EqualError(t, err, "Invalid value: bizbaz")

	// Get the smallest USB target.
	tgt.MinSize = ""
	tgt.Bus = "usb"
	tgt.SortOrder = "Smallest"
	target, size, err := getTargetDevice(devs, tgt)
	require.NoError(t, err)
	require.Equal(t, "/dev/sdc", target)
	require.Equal(t, 52, size)

	// Get the largest NVME target.
	tgt.Bus = "NVME"
	tgt.SortOrder = "LARGEST"
	target, size, err = getTargetDevice(devs, tgt)
	require.NoError(t, err)
	require.Equal(t, "/dev/nvme1n1", target)
	require.Equal(t, 61, size)

	// Get the smallest device with "incus_disk" in its ID.
	tgt.Bus = ""
	tgt.SortOrder = "smallest"
	tgt.ID = "incus_disk"
	target, size, err = getTargetDevice(devs, tgt)
	require.NoError(t, err)
	require.Equal(t, "/dev/sdb", target)
	require.Equal(t, 51, size)

	// Get the smallest device with size of at least 55.
	tgt.SortOrder = "smallest"
	tgt.MinSize = "55"
	target, size, err = getTargetDevice(devs, tgt)
	require.NoError(t, err)
	require.Equal(t, "/dev/nvme0n1", target)
	require.Equal(t, 60, size)

	// Get the largest device with size of no more than 70.
	tgt.SortOrder = "largest"
	tgt.MaxSize = "70"
	target, size, err = getTargetDevice(devs, tgt)
	require.NoError(t, err)
	require.Equal(t, "/dev/nvme1n1", target)
	require.Equal(t, 61, size)

	// Get the largest device with size between 51 and 59.
	tgt.SortOrder = "largest"
	tgt.MaxSize = "59"
	tgt.MinSize = "51"
	target, size, err = getTargetDevice(devs, tgt)
	require.NoError(t, err)
	require.Equal(t, "/dev/sdd", target)
	require.Equal(t, 53, size)
}
