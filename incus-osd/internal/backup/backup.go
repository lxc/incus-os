package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/lxc/incus/v6/shared/revert"

	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

// GetOSBackup returns a tar archive of all the files under /var/lib/incus-os/.
func GetOSBackup() ([]byte, error) {
	// Simplifying assumption: /var/lib/incus-osd/ only contains files that are
	// relatively small. We don't handle traversing directories or need to worry
	// about memory exhaustion when creating the tar archive.
	var ret bytes.Buffer

	tw := tar.NewWriter(&ret)

	files, err := os.ReadDir("/var/lib/incus-os/")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			return nil, errors.New("backup cannot contain directories")
		}

		contents, err := os.ReadFile(filepath.Join("/var/lib/incus-os/", file.Name()))
		if err != nil {
			return nil, err
		}

		header := &tar.Header{
			Name: file.Name(),
			Mode: 0o600,
			Size: int64(len(contents)),
		}

		err = tw.WriteHeader(header)
		if err != nil {
			return nil, err
		}

		_, err = tw.Write(contents)
		if err != nil {
			return nil, err
		}
	}

	err = tw.Close()
	if err != nil {
		return nil, err
	}

	return ret.Bytes(), nil
}

// ApplyOSBackup processes a backup tar archive from the provided io.Reader and performs
// either a partial or complete OS-level restore.
func ApplyOSBackup(ctx context.Context, s *state.State, buf io.Reader, doFullRestore bool) error {
	reverter := revert.New()
	defer reverter.Fail()

	// Backup the current /var/lib/incus-os/.
	err := os.Rename("/var/lib/incus-os/", "/var/lib/incus-os.bak/")
	if err != nil {
		return err
	}

	// If we encounter an error, restore things to the state prior to starting.
	reverter.Add(func() {
		// Restore the backup directory.
		_ = os.RemoveAll("/var/lib/incus-os/")
		_ = os.Rename("/var/lib/incus-os.bak/", "/var/lib/incus-os/")

		// Ensure we load the old state back.
		oldState, _ := state.LoadOrCreate("/var/lib/incus-os/state.txt")
		s = oldState
	})

	// Create a new /var/lib/incus-os/.
	err = os.Mkdir("/var/lib/incus-os/", 0o700)
	if err != nil {
		return err
	}

	// Iterate through each file in the tar archive.
	tr := tar.NewReader(buf)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		if header.Typeflag != tar.TypeReg {
			return errors.New("backup cannot contain anything other than regular files")
		}

		// Don't let someone feed us a path traversal escape attack.
		filename := filepath.Base(header.Name)

		var contents []byte

		_, err = tr.Read(contents)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		// Only restore the local pool key from provided backup when performing a complete system restore.
		if filename == "zpool.local.key" && !doFullRestore {
			// Copy the existing local pool key.
			key, err := os.ReadFile("/var/lib/incus-os.bak/zpool.local.key")
			if err != nil {
				return err
			}

			err = os.WriteFile("/var/lib/incus-os/zpool.local.key", key, 0o600)
			if err != nil {
				return err
			}

			continue
		}

		// Restoring the actual state struct requires additional work.
		if filename == "state.txt" {
			// Decode the state from backup.
			newState := &state.State{}

			err := state.Decode(contents, nil, newState)
			if err != nil {
				return err
			}

			newState.SetPath(filepath.Join("/var/lib/incus-os/", filename))

			// Process the new state and make necessary adjustments to the system
			// so the actual system state matches.
			err = processNewState(ctx, s, newState, doFullRestore)
			if err != nil {
				return errors.New("unable to process state from backup: " + err.Error())
			}

			continue
		}

		// Write any other file to disk.
		err = os.WriteFile(filepath.Join("/var/lib/incus-os/", filename), contents, 0o600)
		if err != nil {
			return err
		}
	}

	// Remove the old /var/lib/incus-os/ backup.
	err = os.RemoveAll("/var/lib/incus-os.bak/")
	if err != nil {
		return err
	}

	reverter.Success()

	// Finally, reboot the system as an easy way to ensure service, network, and application
	// configurations match the new state.
	return systemd.SystemReboot(ctx)
}

func processNewState(ctx context.Context, oldState *state.State, newState *state.State, doFullRestore bool) error {
	// Sanity checks:
	// 1. Need to be able to use TPM to change encryption recovery passphrase(s).
	// 2. At least one recovery passphrase provided.
	// 3. At least one primary application must be installed.
	tpmStatus := secureboot.TPMStatus()
	if tpmStatus != "ok" {
		return errors.New("TPM status isn't OK: " + tpmStatus)
	}

	if len(newState.System.Security.Config.EncryptionRecoveryKeys) == 0 {
		return errors.New("at least one recovery passphrase must be provided")
	}

	havePrimaryApp := false

	for app := range newState.Applications {
		if app == "incus" || app == "migration-manager" || app == "operations-center" {
			havePrimaryApp = true

			break
		}
	}

	if !havePrimaryApp {
		return errors.New("backup state doesn't include the incus, migration-manager, or operations-center application")
	}

	// Make sure list of configured applications is consistent with the new state.
	for oldApp := range oldState.Applications {
		// Uninstall any application that's not present in the new state.
		_, exists := newState.Applications[oldApp]
		if !exists {
			// TODO uninstall app
		}
	}

	for newApp := range newState.Applications {
		// Install any application that's not currently installed.
		_, exists := oldState.Applications[newApp]
		if !exists {
			// TODO install app
		}
	}

	// Filter out unneeded/unwanted fields from the new struct.
	// TODO
	//newState.SecureBoot
	//newState.OS

	// As the last step, reset the recovery passphrase(s) based on what's in the new state.
	// This is done at the end, since we really don't want to try to handle reverting the
	// new passphrase(s) to the old ones if some other part of the backup restore process failed.
	luksVolumes, err := util.GetLUKSVolumePartitions()
	if err != nil {
		return err
	}

	for _, volume := range luksVolumes {
		err := systemd.WipeAllRecoveryKeys(ctx, volume)
		if err != nil {
			return err
		}
	}

	newKeys := newState.System.Security.Config.EncryptionRecoveryKeys
	newState.System.Security.Config.EncryptionRecoveryKeys = []string{}

	for _, key := range newKeys {
		err := systemd.AddEncryptionKey(ctx, newState, key)
		if err != nil {
			return err
		}
	}

	// Now, replace the current state in-memory and write it to disk.
	oldState = newState

	return oldState.Save()
}
