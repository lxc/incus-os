package systemd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/foxboron/go-uefi/authenticode"
	"github.com/lxc/incus/v6/shared/subprocess"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

// ErrReleaseNotFound is returned when the os-release file can't be located.
var ErrReleaseNotFound = errors.New("couldn't determine current OS release")

// GetCurrentRelease returns the current NAME and IMAGE_VERSION from the os-release file.
func GetCurrentRelease(_ context.Context) (string, string, error) {
	// Open the os-release file.
	fd, err := os.Open("/lib/os-release")
	if err != nil {
		return "", "", err
	}

	defer fd.Close()

	name := ""
	version := ""

	// Prepare reader.
	fdScan := bufio.NewScanner(fd)
	for fdScan.Scan() {
		line := fdScan.Text()
		fields := strings.SplitN(line, "=", 2)

		if len(fields) != 2 {
			continue
		}

		switch fields[0] {
		case "NAME":
			name = strings.Trim(fields[1], "\"")
		case "IMAGE_VERSION":
			version = strings.Trim(fields[1], "\"")
		default:
		}
	}

	if name != "" && version != "" {
		return name, version, nil
	}

	return "", "", ErrReleaseNotFound
}

// ApplySystemUpdate instructs systemd-sysupdate to apply any pending update and optionally reboot the system.
func ApplySystemUpdate(ctx context.Context, luksPassword string, version string, reboot bool) error {
	// WORKAROUND: Start the boot.mount unit so /boot autofs is active before we create a new mount namespace.
	err := StartUnit(ctx, "boot.mount")
	if err != nil {
		return err
	}

	// Determine Secure Boot state.
	sbEnabled, err := secureboot.Enabled()
	if err != nil {
		return err
	}

	var newUKIFile string

	var newUsrImageFile string

	var newUsrImageVeritySigFile string

	updateFiles, err := os.ReadDir(SystemUpdatesPath)
	if err != nil {
		return err
	}

	for _, file := range updateFiles {
		if strings.HasSuffix(file.Name(), "_"+version+".efi") { //nolint:gocritic
			newUKIFile = filepath.Join(SystemUpdatesPath, file.Name())
		} else if strings.Contains(file.Name(), "_"+version+".usr-x86-64.") || strings.Contains(file.Name(), "_"+version+".usr-arm64.") {
			newUsrImageFile = filepath.Join(SystemUpdatesPath, file.Name())
		} else if strings.Contains(file.Name(), "_"+version+".usr-x86-64-verity-sig.") || strings.Contains(file.Name(), "_"+version+".usr-arm64-verity-sig.") {
			newUsrImageVeritySigFile = filepath.Join(SystemUpdatesPath, file.Name())
		}
	}

	if newUKIFile == "" {
		return errors.New("unable to find UKI file for system update version " + version)
	}

	if newUsrImageFile == "" {
		return errors.New("unable to find usr image file for system update version " + version)
	}

	if newUsrImageVeritySigFile == "" {
		return errors.New("unable to find usr verity signature file for system update version " + version)
	}

	// Verify that the UKI and usr image file are signed by a trusted certificate.
	sigFile, err := os.Open(newUsrImageVeritySigFile) //nolint:gosec
	if err != nil {
		return err
	}
	defer sigFile.Close()

	buf, err := io.ReadAll(sigFile)
	if err != nil {
		return err
	}

	// Decode the json metadata.
	metadata := veritySignatureMetadata{}
	buf = bytes.Trim(buf, "\x00")

	err = json.Unmarshal(buf, &metadata)
	if err != nil {
		return err
	}

	// Get the trusted certificate that matches the verity certificate fingerprint.
	trustedCert, err := getTrustedVerityCertificate(ctx, metadata.CertificateFingerprint)
	if err != nil {
		return err
	}

	// Verify the UKI image.
	ukiImage, err := os.Open(newUKIFile) //nolint:gosec
	if err != nil {
		return err
	}
	defer ukiImage.Close()

	ukiAuthenticode, err := authenticode.Parse(ukiImage)
	if err != nil {
		return err
	}

	_, err = ukiAuthenticode.Verify(trustedCert)
	if err != nil {
		return err
	}

	// Verify the usr image PKCS7 signature.
	err = verifySignature(metadata.Signature, metadata.RootHash, trustedCert)
	if err != nil {
		return err
	}

	// Check if the Secure Boot key has changed; if it has apply the necessary updates.
	secureBootKeyChanged, err := secureboot.UKIHasDifferentSecureBootCertificate(newUKIFile)
	if err != nil {
		return err
	}

	if secureBootKeyChanged {
		// If the signing key has changed, perform a full encryption rebinding.
		err := secureboot.HandleSecureBootKeyChange(ctx, luksPassword, newUKIFile, newUsrImageFile)
		if err != nil {
			return err
		}
	} else if !sbEnabled {
		// If Secure Boot is disabled, we always must update the PCR4 bindings for the new UKI.
		err := secureboot.UpdatePCR4Binding(ctx, newUKIFile)
		if err != nil {
			return err
		}
	}

	// WORKAROUND: Needed until systemd-sysupdate can be run with system extensions applied.
	cmd := "mount /dev/mapper/usr /usr && /usr/lib/systemd/systemd-sysupdate update " + version
	if reboot {
		cmd += "&& /usr/lib/systemd/systemd-sysupdate reboot"
	}

	_, err = subprocess.RunCommandContext(ctx, "unshare", "-m", "--", "sh", "-c", cmd)
	if err != nil {
		return err
	}

	// Flush all writes to get a consistent ESP if the system gets forcefully rebooted by the user.
	unix.Sync()

	if reboot {
		// Wait 10s to allow time for the system to reboot.
		time.Sleep(10 * time.Second)
	}

	return nil
}
