package recovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus/v7/shared/osarch"
	"github.com/lxc/incus/v7/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/api"
	apiupdate "github.com/lxc/incus-os/incus-osd/api/images"
	"github.com/lxc/incus-os/incus-osd/certs"
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

	// Run the hotfix script, if any.
	err = runHotfix(ctx, mountDir)
	if err != nil {
		return err
	}

	// Apply the update(s), if any.
	err = applyUpdate(ctx, s, mountDir)
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
		// If a signed script isn't present, nothing to do.
		return nil //nolint:nilerr
	}

	scriptContents, err := os.ReadFile(filepath.Join(mountDir, "hotfix.sh.sig"))
	if err != nil {
		return err
	}

	output, err := RunSignedScript(ctx, scriptContents)
	if err != nil {
		return err
	}

	if output != "" {
		slog.InfoContext(ctx, "Hotfix script completed", "output", output)
	}

	return nil
}

// RunSignedScript verifies and executes a signed hotfix script, returning the script output and any error.
func RunSignedScript(ctx context.Context, signedScript []byte) (string, error) {
	// Do a quick check that the input looks like it's been properly S/MIME-signed.
	if !bytes.Contains(signedScript, []byte("Content-Type: multipart/signed; protocol=\"application/x-pkcs7-signature\";")) {
		return "", errors.New("doesn't look like S/MIME-signed input")
	}

	slog.InfoContext(ctx, "Hotfix script detected, verifying signature")

	// Load the embedded certificates.
	embeddedCerts, err := certs.GetEmbeddedCertificates()
	if err != nil {
		return "", err
	}

	// Validate the signed hotfix script using the Support intermediate CA.
	verified, err := util.VerifySMIME(ctx, []*x509.Certificate{embeddedCerts.SupportCACertificate}, signedScript)
	if err != nil {
		return "", err
	}

	// Write the script contents to a temp file.
	scriptFile, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}

	defer os.Remove(scriptFile.Name())

	_, err = scriptFile.WriteString(strings.ReplaceAll(verified.String(), "\r\n", "\n"))
	if err != nil {
		return "", err
	}

	err = scriptFile.Chmod(0o755)
	if err != nil {
		return "", err
	}

	err = scriptFile.Close()
	if err != nil {
		return "", err
	}

	slog.InfoContext(ctx, "Running hotfix script")

	// Run the hotfix script.
	output, err := subprocess.RunCommandContext(ctx, scriptFile.Name())

	return output, err
}

func applyUpdate(ctx context.Context, s *state.State, mountDir string) error {
	updateDir := filepath.Join(mountDir, "update")

	// Check if update.sjson exists.
	_, err := os.Stat(filepath.Join(updateDir, "update.sjson"))
	if err != nil {
		return nil
	}

	slog.InfoContext(ctx, "Update metadata detected, verifying signature")

	updateContents, err := os.ReadFile(filepath.Join(updateDir, "update.sjson"))
	if err != nil {
		return err
	}

	// Load the embedded certificates.
	embeddedCerts, err := certs.GetEmbeddedCertificates()
	if err != nil {
		return err
	}

	// Validate the signed update json using either the Update intermediate CA.
	verified, err := util.VerifySMIME(ctx, []*x509.Certificate{embeddedCerts.UpdateCACertificate}, updateContents)
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

	// Make sure the path used by the debug provider exists.
	err = os.MkdirAll(providers.DebugPath, 0o700)
	if err != nil {
		return err
	}

	// Ensure we cleanup after ourselves.
	defer os.RemoveAll(providers.DebugPath)

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

		// Verify the SHA256 of each file that exists before making it available to the debug provider.
		err := verifyAndDecompressFile(updateDir, file)
		if err != nil {
			if os.IsNotExist(err) {
				slog.WarnContext(ctx, "Skipping missing file: '"+file.Filename+"'")

				continue
			}

			return err
		}
	}

	// Set the RELEASE version for the debug provider.
	r, err := os.Create(filepath.Join(providers.DebugPath, "RELEASE"))
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = r.WriteString(updateInfo.Version + "\n")
	if err != nil {
		return err
	}

	// Stash the current provider state and configuration. We'll be forcing the system to use the debug provider,
	// and once we're done we want to restore the actual provider configuration.
	currentProvider := s.System.Provider

	defer func() { s.System.Provider = currentProvider }()

	s.System.Provider = api.SystemProvider{
		Config: api.SystemProviderConfig{
			Name: "debug",
		},
	}

	p, err := providers.Load(ctx, s, false)
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
	tfd, err := os.Create(filepath.Join(providers.DebugPath, filepath.Base(strings.TrimSuffix(file.Filename, ".gz"))))
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
