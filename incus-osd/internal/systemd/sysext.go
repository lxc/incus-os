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

// RemoveExtension removes the specified system extension layer.
func RemoveExtension(ctx context.Context, name string) error {
	// Remove the sysext image.
	err := os.Remove(filepath.Join(SystemExtensionsPath, name+".raw"))
	if err != nil {
		return err
	}

	// Refresh the system extensions.
	return RefreshExtensions(ctx)
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
