package reset

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/install"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
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

	for seed, seedData := range resetSeed.Seeds {
		if seedData == nil {
			return errors.New("seed data for '" + seed + "' is not defined")
		}

		tmp := empty{}

		err := json.Unmarshal(seedData, &tmp)
		if err != nil {
			return err
		}
	}

	// First, write the seed data. Do this first, since any errors encountered won't cause
	// the currently running system to break.
	partitionPrefix := install.GetPartitionPrefix(underlyingDevice)
	seedPartition := underlyingDevice + partitionPrefix + "2"
	seeds := make(map[string][]byte)

	if !resetSeed.WipeExistingSeeds {
		// Read any existing seed data and augment it with provided seed update(s), if any.
		existingSeeds, err := getExistingSeeds(seedPartition)
		if err != nil {
			return err
		}

		for seed, seedData := range resetSeed.Seeds {
			// Clear any existing data for the new seed.
			for _, ext := range []string{".json", ".yaml", ".yml"} {
				delete(existingSeeds, seed+ext)
			}

			// Copy seed data and set appropriate file extension.
			existingSeeds[seed+".json"] = seedData
		}

		seeds = existingSeeds
	} else {
		for seed, seedData := range resetSeed.Seeds {
			// Copy seed data and set appropriate file extension.
			seeds[seed+".json"] = seedData
		}
	}

	// #nosec G304
	f, err := os.Create(seedPartition)
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)

	for seed, seedData := range seeds {
		header := &tar.Header{
			Name: seed,
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

	// Beyond this point, we start making destructive changes to the system.
	// If an error is encountered, we'll likely end up with a bricked system.

	// Second, wipe the TPM.
	_, err = subprocess.RunCommandContext(ctx, "tpm2_clear")
	if err != nil {
		// Some systems return errors when trying to clear the TPM. As a workaround,
		// allow the user to indicate we should accept this error and continue.
		if !resetSeed.AllowTPMResetFailure {
			return err
		}
	}

	// Third, wipe system partitions (swap, root, and local-data).
	for _, partitionIndex := range []string{"9", "10", "11"} {
		_, err := subprocess.RunCommandContext(ctx, "sgdisk", "-d", partitionIndex, underlyingDevice)
		if err != nil {
			return err
		}
	}

	// Finally, sync disks and immediately reboot the system.
	unix.Sync()

	// Spawn a go routine which will sleep one second before force-rebooting the system.
	// This allows the HTTP connection to the client to properly close.
	go func() {
		time.Sleep(1 * time.Second)

		_ = os.WriteFile("/proc/sysrq-trigger", []byte("b"), 0o600)
	}()

	return nil
}

func getExistingSeeds(seedPartition string) (map[string][]byte, error) {
	ret := make(map[string][]byte)

	// #nosec G304
	f, err := os.Open(seedPartition)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		ret[header.Name] = make([]byte, header.Size)

		bytesRead, err := tr.Read(ret[header.Name])
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		if int64(bytesRead) != header.Size {
			return nil, fmt.Errorf("only read %d of %d bytes for existing seed file '%s'", bytesRead, header.Size, header.Name)
		}
	}

	return ret, nil
}
