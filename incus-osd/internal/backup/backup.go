package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/lxc/incus/v6/shared/revert"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
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

		fd, err := os.Open(filepath.Join("/var/lib/incus-os/", file.Name()))
		if err != nil {
			return nil, err
		}
		defer fd.Close() //nolint:revive

		stat, err := fd.Stat()
		if err != nil {
			return nil, err
		}

		header := &tar.Header{
			Name: file.Name(),
			Mode: 0o600,
			Size: stat.Size(),
		}

		err = tw.WriteHeader(header)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(tw, fd)
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
// an OS-level restore. If specific skip options are supplied, some parts of the backup
// may be omitted.
func ApplyOSBackup(ctx context.Context, s *state.State, buf io.Reader, skipOptions []string) error {
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
	stateSuccessfullyProcessed := false

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

		// If told to skip restoring local pool key, copy the existing one from the backup directory.
		if filename == "zpool.local.key" && slices.Contains(skipOptions, "local-data-encryption-key") {
			// Copy the existing local pool key.
			oldKey, err := os.Open("/var/lib/incus-os.bak/zpool.local.key")
			if err != nil {
				return err
			}
			defer oldKey.Close() //nolint:revive

			newKey, err := os.Create("/var/lib/incus-os/zpool.local.key")
			if err != nil {
				return err
			}
			defer newKey.Close() //nolint:revive

			_, err = io.Copy(newKey, oldKey)
			if err != nil {
				return err
			}

			continue
		}

		// Write file to disk.
		// #nosec G304
		fd, err := os.Create(filepath.Join("/var/lib/incus-os/", filename))
		if err != nil {
			return err
		}
		defer fd.Close() //nolint:revive

		// Read from the archive in chunks to avoid excessive memory consumption.
		var size int64

		for {
			n, err := io.CopyN(fd, tr, 4*1024*1024)
			size += n

			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				return err
			}
		}

		// Restoring the actual state struct requires additional work.
		if filename == "state.txt" {
			// Decode the state from backup.
			newState, err := state.LoadOrCreate("/var/lib/incus-os/state.txt")
			if err != nil {
				return err
			}

			// Process the new state and make necessary adjustments to the system
			// so the actual system state matches.
			err = processNewState(ctx, &s, newState, skipOptions)
			if err != nil {
				return errors.New("unable to process state from backup: " + err.Error())
			}

			// Flag that we successfully read the state file from the backup; if not
			// present the system would reboot into an empty state which isn't what we want.
			stateSuccessfullyProcessed = true
		}
	}

	if !stateSuccessfullyProcessed {
		return errors.New("failed to read state.txt from backup")
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

func processNewState(ctx context.Context, oldState **state.State, newState *state.State, skipOptions []string) error {
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
	for oldApp := range (*oldState).Applications {
		// Uninstall any application that's not present in the new state.
		_, exists := newState.Applications[oldApp]
		if !exists {
			err := uninstallApplication(ctx, *oldState, oldApp)
			if err != nil {
				return err
			}
		}
	}

	for newApp, a := range newState.Applications {
		// Install any application that's not currently installed.
		_, exists := (*oldState).Applications[newApp]
		if !exists {
			// Get the provider.
			p, err := providers.Load(ctx, newState)
			if err != nil {
				return err
			}

			appVersion, err := installApplication(ctx, newState, p, newApp)
			if err != nil {
				return err
			}

			// Update the application's state.
			a.State.Version = appVersion
			a.State.Initialized = true
			newState.Applications[newApp] = a
		}
	}

	// Copy over relevant current state.
	newState.SecureBoot = (*oldState).SecureBoot
	newState.OS = (*oldState).OS

	// Clear any stale state from the new struct.
	newState.Services.Ceph.State = struct{}{}
	newState.Services.ISCSI.State = api.ServiceISCSIState{}
	newState.Services.LVM.State = api.ServiceLVMState{}
	newState.Services.Multipath.State = api.ServiceMultipathState{}
	newState.Services.NVME.State = api.ServiceNVMEState{}
	newState.Services.OVN.State = struct{}{}
	newState.Services.USBIP.State = struct{}{}
	newState.System.Logging.State = struct{}{}
	newState.System.Network.State = api.SystemNetworkState{}
	newState.System.Provider.State = api.SystemProviderState{}
	newState.System.Security.State = api.SystemSecurityState{}
	newState.System.Update.State = api.SystemUpdateState{}

	// If instructed to skip restoring network MACs, replace any value with the Interface
	// or Bond name, which will be dynamically resolved to the actual device's MAC when
	// the system restarts.
	if slices.Contains(skipOptions, "network-macs") {
		for i := range newState.System.Network.Config.Interfaces {
			if newState.System.Network.Config.Interfaces[i].Hwaddr != "" {
				newState.System.Network.Config.Interfaces[i].Hwaddr = newState.System.Network.Config.Interfaces[i].Name
			}
		}

		for i := range newState.System.Network.Config.Bonds {
			if newState.System.Network.Config.Bonds[i].Hwaddr != "" {
				newState.System.Network.Config.Bonds[i].Hwaddr = newState.System.Network.Config.Bonds[i].Name
			}
		}
	}

	if !slices.Contains(skipOptions, "encryption-recovery-keys") {
		// As the final step, reset the recovery passphrase(s) based on what's in the new state.
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
	}

	// Now, replace the current state in-memory and write it to disk.
	*oldState = newState

	return (*oldState).Save()
}

func uninstallApplication(ctx context.Context, s *state.State, appName string) error {
	// Load the existing application.
	app, err := applications.Load(ctx, s, appName)
	if err != nil {
		return err
	}

	// Stop the application.
	err = app.Stop(ctx, "")
	if err != nil {
		return err
	}

	// Wipe local data.
	err = app.WipeLocalData()
	if err != nil {
		return err
	}

	// Remove the sysext layer.
	return systemd.RemoveExtension(ctx, appName)
}

func installApplication(ctx context.Context, s *state.State, p providers.Provider, appName string) (string, error) {
	// Fetch the application from provider.
	papp, err := p.GetApplication(ctx, appName)
	if err != nil {
		return "", err
	}

	// Download the application.
	err = papp.Download(ctx, systemd.SystemExtensionsPath, nil)
	if err != nil {
		return "", err
	}

	// Verify the application is signed with a trusted key in the kernel's keyring.
	err = systemd.VerifyExtensionCertificateFingerprint(ctx, filepath.Join(systemd.SystemExtensionsPath, papp.Name()+".raw"))
	if err != nil {
		return "", err
	}

	// Reload sysext layer.
	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return "", err
	}

	// Get the application.
	aapp, err := applications.Load(ctx, s, appName)
	if err != nil {
		return "", err
	}

	// Start the application.
	err = aapp.Start(ctx, papp.Version())
	if err != nil {
		return "", err
	}

	// Initialize the application.
	err = aapp.Initialize(ctx)
	if err != nil {
		return "", err
	}

	return papp.Version(), nil
}
