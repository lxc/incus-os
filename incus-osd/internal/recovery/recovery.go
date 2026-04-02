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
	"strings"

	"github.com/lxc/incus/v6/shared/osarch"
	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/update"
	"github.com/lxc/incus-os/incus-osd/internal/util"
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

	// Workaround for recovery running on first boot when no provider has been set yet.
	if s.System.Provider.Config.Name == "" {
		s.System.Provider.Config.Name = "images"

		defer func() { s.System.Provider.Config.Name = "" }()
	}

	// Get the expected CA certificate to validate the update metadata.
	updateCA, err := providers.GetUpdateCACert()
	if err != nil {
		return err
	}

	// Run the hotfix script, if any.
	err = runHotfix(ctx, updateCA, mountDir)
	if err != nil {
		return err
	}

	// Apply the update(s), if any.
	err = applyUpdate(ctx, s, updateCA, mountDir)
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

	f, err := os.Open(filepath.Join(mountDir, "hotfix.sh.sig"))
	if err != nil {
		return err
	}
	defer f.Close()

	// Validate the signed hotfix script.
	verified, err := util.VerifySMIME(ctx, updateCA, f)
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

func applyUpdate(ctx context.Context, s *state.State, updateCA string, mountDir string) error {
	updateDir := filepath.Join(mountDir, "update")

	// Check if update.sjson exists.
	_, err := os.Stat(filepath.Join(updateDir, "update.sjson"))
	if err != nil {
		return nil
	}

	slog.InfoContext(ctx, "Update metadata detected, verifying signature")

	f, err := os.Open(filepath.Join(updateDir, "update.sjson"))
	if err != nil {
		return err
	}
	defer f.Close()

	// Validate the signed update.
	verified, err := util.VerifySMIME(ctx, updateCA, f)
	if err != nil {
		return err
	}

	// Parse the update.
	updateInfo := &apiupdate.Update{}

	err = json.NewDecoder(bytes.NewReader(verified.Bytes())).Decode(updateInfo)
	if err != nil {
		return err
	}

	// Refuse to apply any updates that are older than the currently running version.
	if strings.Compare(updateInfo.Version, s.OS.RunningRelease) < 0 {
		return errors.New("refusing to apply update version (" + updateInfo.Version + ") that is older than the current running " + s.OS.Name + " version")
	}

	slog.InfoContext(ctx, "Processing validated update metadata", "version", updateInfo.Version)

	// Make sure the path used by the local provider exists.
	err = os.MkdirAll(providers.LocalPath, 0o700)
	if err != nil {
		return err
	}

	// Ensure we cleanup after ourselves.
	defer os.RemoveAll(providers.LocalPath)

	// Get local architecture.
	archName, err := osarch.ArchitectureGetLocal()
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "Decompressing and verifying each update file")

	for _, file := range updateInfo.Files {
		// Skip files not for our architecture.
		if file.Architecture != "" && string(file.Architecture) != archName {
			continue
		}

		// Verify the SHA256 of each file that exists before making it available to the local provider.
		err := verifyAndDecompressFile(updateDir, file)
		if err != nil {
			return err
		}
	}

	// Set the RELEASE version for the local provider.
	r, err := os.Create(filepath.Join(providers.LocalPath, "RELEASE"))
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = r.WriteString(updateInfo.Version + "\n")
	if err != nil {
		return err
	}

	// Stash the current provider state and configuration. We'll be forcing the system to use the local provider,
	// and once we're done we want to restore the actual provider configuration.
	currentProvider := s.System.Provider

	defer func() { s.System.Provider = currentProvider }()

	s.System.Provider = api.SystemProvider{
		Config: api.SystemProviderConfig{
			Name: "local",
		},
	}

	p, err := providers.Load(ctx, s)
	if err != nil {
		return err
	}

	// Trigger an update check.
	update.Checker(ctx, s, p, true, false)

	return nil
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
	tfd, err := os.Create(filepath.Join(providers.LocalPath, filepath.Base(strings.TrimSuffix(file.Filename, ".gz"))))
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
