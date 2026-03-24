package systemd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"
)

// RefreshExtensions causes systemd-sysext to re-scan and reload the system extensions.
func RefreshExtensions(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysext", "refresh")
	if err != nil {
		return err
	}

	// Reload the systemd daemon.
	return ReloadDaemon(ctx)
}

// RemoveExtension removes all versions of the specified system extension image from disk.
func RemoveExtension(ctx context.Context, name string) error {
	// Remove symlink from /var/lib/extensions/.
	err := os.Remove(filepath.Join(SystemExtensionsPath, name+".raw"))
	if err != nil {
		return err
	}

	// Prepare to remove any version of the sysext image that exists on disk.
	dirEntries, err := os.ReadDir(LocalExtensionsPath)
	if err != nil {
		return err
	}

	// Iterate through each directory under /var/lib/incus-os-extensions/, which
	// corresponds to the version of one or more installed applications.
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// Check if an application image exists, and if so, remove it.
			_, err := os.Stat(filepath.Join(LocalExtensionsPath, entry.Name(), name+".raw"))
			if err == nil {
				err := os.Remove(filepath.Join(LocalExtensionsPath, entry.Name(), name+".raw"))
				if err != nil {
					return err
				}

				// Opportunistically attempt to remove the directory. This will fail
				// if it is non-empty, which is OK.
				_ = os.Remove(filepath.Join(LocalExtensionsPath, entry.Name()))
			}
		}
	}

	// Reload the extensions.
	return reloadExtensions(ctx)
}

// VerifyExtension takes the filename of a sysext image and verifies its basic format is correct,
// that its certificate fingerprint matches one currently trusted by the kernel, and that the signature
// can be verified by the trusted certificate. systemd-sysext performs similar tasks, but we do this
// by hand to catch potential issues when Secure Boot is disabled _before_ overwriting the existing
// known good sysext image.
func VerifyExtension(ctx context.Context, extensionFile string) error {
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

	// Now that we have a trusted certificate, verify the PKCS7 signature of the root hash.
	return verifySignature(metadata.Signature, metadata.RootHash, trustedCert)
}

func reloadExtensions(ctx context.Context) error {
	// Refresh the installed sysext images.
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysext", "refresh")
	if err != nil {
		return err
	}

	// Reload the systemd daemon.
	return ReloadDaemon(ctx)
}
