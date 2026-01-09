package recovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// CheckRunRecovery checks if a partition labeled "RESCUE_DATA" is present. If so,
// and if the filesystem is vfat or iso9660, it will mount the partition and first
// run any hotfix.sh script, then apply any updates in the update/ folder. Both
// the hotfix script and update metadata is verified to have been properly signed
// by the expected certificate.
func CheckRunRecovery(ctx context.Context, s *state.State) error {
	device := "/dev/disk/by-partlabel/RESCUE_DATA"

	// Check if a recovery partition exists.
	_, err := os.Stat(device)
	if err != nil {
		_, err := os.Stat("/dev/disk/by-label/RESCUE_DATA")
		if err != nil {
			return nil
		}

		device = "/dev/disk/by-label/RESCUE_DATA"
	}

	slog.InfoContext(ctx, "Recovery partition detected")

	// Mount the recovery partition.
	mountDir, err := os.MkdirTemp("", "incus-os-recovery")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountDir)

	// Try to mount as vfat
	err = unix.Mount(device, mountDir, "vfat", 0, "ro")
	if err != nil {
		// Try to mount as iso9660
		err = unix.Mount(device, mountDir, "iso9660", 0, "ro")
		if err != nil {
			return errors.New("unable to mount recovery partition as vfat or iso9660")
		}
	}
	defer unix.Unmount(mountDir, 0)

	// Get the expected CA certificate to validate the update metadata.
	p, err := providers.Load(ctx, s)
	if err != nil {
		return err
	}

	updateCA, err := p.GetSigningCACert()
	if err != nil {
		return err
	}

	// Run the hotfix script, if any.
	err = runHotfix(ctx, updateCA, mountDir)
	if err != nil {
		return err
	}

	// Apply the update(s), if any.
	apps := []string{}

	for app := range s.Applications {
		apps = append(apps, app)
	}

	// Similar to normal startup logic, if no applications are installed default
	// to selecting incus.
	if len(apps) == 0 {
		apps = append(apps, "incus")
	}

	err = applyUpdate(ctx, s, updateCA, mountDir, apps, s.System.Security.Config.EncryptionRecoveryKeys[0])
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Recovery actions completed")

	return nil
}

func runHotfix(ctx context.Context, updateCA string, mountDir string) error {
	// Check if hotfix.sh.sig exists.
	_, err := os.Stat(filepath.Join(mountDir, "hotfix.sh.sig"))
	if err != nil {
		return nil
	}

	slog.InfoContext(ctx, "Hotfix script detected, verifying signature")

	// Write the CA certificate.
	rootCA, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}

	defer os.Remove(rootCA.Name())

	_, err = rootCA.WriteString(updateCA)
	if err != nil {
		return err
	}

	// Validate the signed hotfix script.
	verified := bytes.NewBuffer(nil)

	err = subprocess.RunCommandWithFds(ctx, nil, verified, "openssl", "smime", "-verify", "-text", "-CAfile", rootCA.Name(), "-in", filepath.Join(mountDir, "hotfix.sh.sig"))
	if err != nil {
		return err
	}

	// Write the script contents to a temp file.
	scriptFile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}

	defer os.Remove(scriptFile.Name())

	_, err = scriptFile.WriteString(strings.ReplaceAll(verified.String(), "\r\n", "\n"))
	if err != nil {
		return err
	}

	err = scriptFile.Chmod(0o755)
	if err != nil {
		return err
	}

	err = scriptFile.Close()
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Running hotfix script")

	// Run the hotfix script.
	_, err = subprocess.RunCommandContext(ctx, scriptFile.Name())

	return err
}

func applyUpdate(ctx context.Context, s *state.State, updateCA string, mountDir string, installedApplications []string, luksPassword string) error {
	updateDir := filepath.Join(mountDir, "update")

	// Check if update.sjson exists.
	_, err := os.Stat(filepath.Join(updateDir, "update.sjson"))
	if err != nil {
		return nil
	}

	slog.InfoContext(ctx, "Update metadata detected, verifying signature")

	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return err
	}

	// Write the CA certificate.
	rootCA, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}

	defer os.Remove(rootCA.Name())

	_, err = rootCA.WriteString(updateCA)
	if err != nil {
		return err
	}

	// Validate the signed update.
	verified := bytes.NewBuffer(nil)

	err = subprocess.RunCommandWithFds(ctx, nil, verified, "openssl", "smime", "-verify", "-text", "-CAfile", rootCA.Name(), "-in", filepath.Join(updateDir, "update.sjson"))
	if err != nil {
		return err
	}

	// Parse the update.
	update := &apiupdate.Update{}

	err = json.NewDecoder(bytes.NewReader(verified.Bytes())).Decode(update)
	if err != nil {
		return err
	}

	if len(update.Files) == 0 {
		return errors.New("no files in update")
	}

	// Refuse to apply any updates that are older than the currently running versions.
	if providers.DatetimeComparison(s.OS.RunningRelease, update.Version) {
		return errors.New("refusing to apply update version (" + update.Version + ") that is older than the current running version")
	}

	for _, dir := range []string{systemd.SystemExtensionsPath, systemd.SystemUpdatesPath} {
		// Clear the path.
		err := os.RemoveAll(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// Create the directory.
		err = os.MkdirAll(dir, 0o700)
		if err != nil {
			return err
		}
	}

	// Verify the SHA256 of each file that exists and copy files to expected install location.
	slog.InfoContext(ctx, "Decompressing and verifying each update file")

	for _, file := range update.Files {
		// Only process files that match our architecture.
		if string(file.Architecture) != archName {
			continue
		}

		// Only process OS or application updates.
		if file.Type != apiupdate.UpdateFileTypeUpdateEFI && file.Type != apiupdate.UpdateFileTypeUpdateUsr && file.Type != apiupdate.UpdateFileTypeUpdateUsrVerity && file.Type != apiupdate.UpdateFileTypeUpdateUsrVeritySignature && file.Type != apiupdate.UpdateFileTypeApplication {
			continue
		}

		// Don't process any applications that are not already installed.
		if file.Type == apiupdate.UpdateFileTypeApplication {
			if !slices.Contains(installedApplications, filepath.Base(strings.TrimSuffix(file.Filename, ".raw.gz"))) {
				continue
			}
		}

		err := verifyAndDecompressFile(updateDir, file)
		if err != nil {
			return err
		}
	}

	// Refresh the applications.
	slog.InfoContext(ctx, "Applying application update(s)")

	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return err
	}

	// Ensure that each application installed/updated as part of the recovery
	// action is properly recorded in the state.
	for _, appName := range installedApplications {
		app, ok := s.Applications[appName]
		if !ok {
			// Add the application to the state, then let normal startup
			// logic handle starting and initializing after the recovery
			// actions are complete.
			s.Applications[appName] = api.Application{
				State: api.ApplicationState{
					Initialized: false,
					Version:     update.Version,
				},
			}
		} else {
			// Update the existing application's version.
			app.State.Version = update.Version
			s.Applications[appName] = app
		}
	}

	// Apply the OS update.
	slog.InfoContext(ctx, "Applying OS update(s)")

	err = systemd.ApplySystemUpdate(ctx, luksPassword, update.Version, false)
	if err != nil {
		return err
	}

	// Record the newly installed OS version.
	s.OS.NextRelease = update.Version
	s.System.Update.State.NeedsReboot = s.OS.RunningRelease != s.OS.NextRelease

	return s.Save()
}

func verifyAndDecompressFile(updateDir string, file apiupdate.UpdateFile) error {
	// #nosec G304
	fd, err := os.Open(filepath.Join(updateDir, file.Filename))
	if err != nil {
		return err
	}
	defer fd.Close()

	h := sha256.New()

	_, err = io.Copy(h, fd)
	if err != nil {
		return err
	}

	if file.Sha256 != hex.EncodeToString(h.Sum(nil)) {
		return errors.New("sha256 mismatch for file " + file.Filename)
	}

	// Decompress and copy verified update files to their expected locations.
	targetPath := systemd.SystemUpdatesPath
	if file.Type == apiupdate.UpdateFileTypeApplication {
		targetPath = systemd.SystemExtensionsPath
	}

	// Reset back to the beginning of each file.
	_, err = fd.Seek(0, 0)
	if err != nil {
		return err
	}

	// Setup a gzip reader to decompress file contents.
	gz, err := gzip.NewReader(fd)
	if err != nil {
		return err
	}

	defer gz.Close()

	// Create the target path.
	// #nosec G304
	tfd, err := os.Create(filepath.Join(targetPath, filepath.Base(strings.TrimSuffix(file.Filename, ".gz"))))
	if err != nil {
		return err
	}

	defer tfd.Close()

	// Read from the decompressor in chunks to avoid excessive memory consumption.
	for {
		_, err = io.CopyN(tfd, gz, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}
	}

	return nil
}
