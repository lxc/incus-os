package systemd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/internal/keyring"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
)

type sysextMetadata struct {
	RootHash               string `json:"rootHast"`               //nolint:tagliatelle
	CertificateFingerprint string `json:"certificateFingerprint"` //nolint:tagliatelle
	Signature              string `json:"signature"`
}

// RefreshExtensions causes systemd-sysext to re-scan and reload the system extensions.
func RefreshExtensions(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysext", "refresh")
	if err != nil {
		return err
	}

	return nil
}

// RemoveExtension removes the specified system extension layer.
func RemoveExtension(ctx context.Context, name string) error {
	// Remove the sysext image.
	err := os.Remove("/var/lib/extensions/" + name + ".raw")
	if err != nil {
		return err
	}

	// Refresh the system extensions.
	err = RefreshExtensions(ctx)
	if err != nil {
		return err
	}

	// Reload the systemd daemon.
	return ReloadDaemon(ctx)
}

// VerifyExtensionCertificateFingerprint takes the filename of a sysext image and verifies its basic
// format is correct and that its certificate fingerprint matches one currently trusted by the kernel.
// Actual cryptographic validation of the signature is deferred to systemd-sysext.
func VerifyExtensionCertificateFingerprint(ctx context.Context, extensionFile string) error {
	// Start with a quick baseline validation of the image.
	_, err := subprocess.RunCommandContext(ctx, "systemd-dissect", "--validate", extensionFile)
	if err != nil {
		return err
	}

	// Get the offset in the image to read json metadata from.
	output, err := subprocess.RunCommandContext(ctx, "sgdisk", "-p", "-i", "3", extensionFile)
	if err != nil {
		return err
	}

	sectorSizeRegex := regexp.MustCompile(`Sector size \(logical\): (\d+) bytes`)
	partitionFirstSectorRegex := regexp.MustCompile(`First sector: (\d+) \(at .+\)`)
	partitionSizeRegex := regexp.MustCompile(`Partition size: (\d+) sectors \(.+\)`)

	sectorSize, err := strconv.Atoi(sectorSizeRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return err
	}

	partitionFirstSector, err := strconv.Atoi(partitionFirstSectorRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return err
	}

	partitionSize, err := strconv.Atoi(partitionSizeRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return err
	}

	// Read the json metadata.
	// #nosec G304
	imageFile, err := os.Open(extensionFile)
	if err != nil {
		return err
	}
	defer imageFile.Close()

	buf := make([]byte, sectorSize*partitionSize)
	readBytes, err := imageFile.ReadAt(buf, int64(sectorSize*partitionFirstSector))

	if err != nil && !errors.Is(err, io.EOF) {
		return err
	} else if readBytes != sectorSize*partitionSize {
		return fmt.Errorf("only read %d of %d expected bytes of JSON metadata from '%s'", readBytes, sectorSize*partitionSize, extensionFile)
	}

	// Decode the json metadata.
	metadata := sysextMetadata{}
	buf = bytes.Trim(buf, "\x00")

	err = json.Unmarshal(buf, &metadata)
	if err != nil {
		return err
	}

	// Get db Secure Boot certificates.
	certs, err := secureboot.GetCertificatesFromVar("db")
	if err != nil {
		return err
	}

	// Get kernel's trusted platform keys.
	kernelKeys, err := keyring.GetKeys(ctx, keyring.PlatformKeyring)
	if err != nil {
		return err
	}

	// Iterate through Secure Boot certificates to find a match.
	for _, cert := range certs {
		sha256Fp := sha256.Sum256(cert.Raw)

		// The image fingerprint matches a certificate in Secure Boot db.
		if metadata.CertificateFingerprint == hex.EncodeToString(sha256Fp[:]) {
			// Iterate through kernel trusted keys to find a match.
			for _, key := range kernelKeys {
				// It would be much better to match on the SHA1 fingerprint of the certificate, but the value returned
				// from /proc/keys isn't the same as sha1.Sum(cert.Raw), and I can't figure out what data the kernel is
				// using to compute its values. So, instead compare the certificate's first subject name to the kernel's
				// description of the key.
				if key.Description == cert.Subject.Names[0].Value {
					return nil
				}

				// In some cases, the kernel uses a combination of organization and common name.
				if len(cert.Subject.Organization) > 0 && key.Description == cert.Subject.Organization[0]+": "+cert.Subject.CommonName {
					return nil
				}
			}

			return fmt.Errorf("sysext image '%s' is signed by a trusted Secure Boot certificate, but the certificate isn't present in the kernel's keyring (reboot needed?)", extensionFile)
		}
	}

	return fmt.Errorf("sysext image '%s' is not signed by a trusted certificate", extensionFile)
}
