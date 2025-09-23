package reset

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/install"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// PerformOSFactoryReset performs an OS-level factory reset.
// !!! THIS WILL RESULT IN THE DESTRUCTION OF ALL DATA CREATED BY !!!
// !!! IncusOS, ANY APPLICATIONS, AND ANY ZFS DATASETS CREATED IN !!!
// !!! THE "local" POOL.                                          !!!
func PerformOSFactoryReset(ctx context.Context, resetSeed *api.SystemReset) error {
	// systemd v258 introduced the factory-reset.target, which in
	// theory should automate the following steps. However, trixie
	// shipped with systemd v257. Potentially we could use a backported
	// version eventually.

	// The version of systemd-repart in v257 does support a "factory
	// reset" mode; however, it doesn't appear to work properly
	// in our use case. Triggering it via EFI variable causes the
	// initrd-parse-etc service to fail in early boot, and triggering
	// via `systemd-repart --factory-reset=yes --dry-run=no` then
	// rebooting causes dev-gpt\x2dauto\x2droot.device to timeout.

	// Get the underlying device.
	underlyingDevice, err := storage.GetUnderlyingDevice()
	if err != nil {
		return err
	}

	// Verify any provided seed data is valid json.
	type empty struct{}

	for seed, seedData := range resetSeed.Seed {
		if seedData == nil {
			return errors.New("seed data for '" + seed + "' is not defined")
		}

		tmp := empty{}

		err := json.Unmarshal(seedData, &tmp)
		if err != nil {
			return err
		}
	}

	// First, wipe the TPM.
	_, err = subprocess.RunCommandContext(ctx, "tpm2_clear")
	if err != nil {
		return err
	}

	// Second, wipe system partitions (swap, root, and local-data).
	for _, partitionIndex := range []string{"9", "10", "11"} {
		_, err := subprocess.RunCommandContext(ctx, "sgdisk", "-d", partitionIndex, underlyingDevice)
		if err != nil {
			return err
		}
	}

	// Third, write the seed data.
	partitionPrefix := install.GetPartitionPrefix(underlyingDevice)
	// #nosec G304
	f, err := os.Create(underlyingDevice + partitionPrefix + "2")
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)

	for seed, seedData := range resetSeed.Seed {
		header := &tar.Header{
			Name: seed + ".json",
			Mode: 0o600,
			Size: int64(len(seedData)),
		}

		err := tw.WriteHeader(header)
		if err != nil {
			return err
		}

		_, err = tw.Write(seedData)
		if err != nil {
			return err
		}
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	// Finally, immediately reboot the system.
	return systemd.SystemReboot(ctx)
}
