package recovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// CheckRunRecovery checks if a partition labeled "RESCUE_DATA" is present. If so,
// and if the filesystem is vfat or isofs, it will mount the partition and first
// run any hotfix.sh script, then apply any updates in the update/ folder. Both
// the hotfix script and update metadata is verified to have been properly signed
// by the expected certificate.
func CheckRunRecovery(ctx context.Context, s *state.State) error {
	// Check if a recovery partition exists.
	_, err := os.Stat("/dev/disk/by-partlabel/RESCUE_DATA")
	if err != nil {
		return nil
	}

	slog.InfoContext(ctx, "Recovery partition detected")

	// Mount the recovery partition.
	mountDir, err := os.MkdirTemp("", "incus-os-recovery")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mountDir)

	// Try to mount as vfat
	err = unix.Mount("/dev/disk/by-partlabel/RESCUE_DATA", mountDir, "vfat", 0, "ro")
	if err != nil {
		// Try to mount as isofs
		err = unix.Mount("/dev/disk/by-partlabel/RESCUE_DATA", mountDir, "isofs", 0, "ro")
		if err != nil {
			return errors.New("unable to mount recovery partition as vfat or isofs")
		}
	}
	defer unix.Unmount(mountDir, 0)

	// Run the hotfix script, if any.
	err = runHotfix(ctx, mountDir)
	if err != nil {
		return err
	}

	// Apply the update(s), if any.
	apps := []string{}

	for app := range s.Applications {
		apps = append(apps, app)
	}

	err = applyUpdate(ctx, mountDir, apps, s.System.Security.Config.EncryptionRecoveryKeys[0])
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Recovery actions completed")

	return nil
}

func runHotfix(ctx context.Context, mountDir string) error {
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

	_, err = fmt.Fprintf(rootCA, "%s", providers.LXCUpdateCA)
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

	_, err = scriptFile.Write(verified.Bytes())
	if err != nil {
		return err
	}

	err = scriptFile.Chmod(0o755)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Running hotfix script")

	// Run the hotfix script.
	_, err = subprocess.RunCommandContext(ctx, scriptFile.Name())

	return err
}

func applyUpdate(ctx context.Context, mountDir string, installedApplications []string, luksPassword string) error {
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

	_, err = fmt.Fprintf(rootCA, providers.LXCUpdateCA)
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

	// Apply the OS update.
	slog.InfoContext(ctx, "Applying OS update(s)")

	return systemd.ApplySystemUpdate(ctx, luksPassword, update.Version, true)
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
