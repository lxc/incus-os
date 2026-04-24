package systemd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/lxc/incus/v6/shared/subprocess"
)

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

// RefreshExtensions refreshes the installed sysext images.
func RefreshExtensions(ctx context.Context) error {
	_, err := subprocess.RunCommandContext(ctx, "systemd-sysext", "refresh")
	if err != nil {
		// Check if we encountered a corrupt sysext image.
		corruptSysextRegex := regexp.MustCompile(`Failed to read metadata for image (.+): Package not installed`)
		match := corruptSysextRegex.FindStringSubmatch(err.Error())

		if len(match) != 2 {
			return err
		}

		// Attempt to delete any corrupt sysext images that exist on-disk.
		err := removeCorruptSysext(ctx, match[1])
		if err != nil {
			return err
		}
	}

	// Reload the systemd daemon.
	return ReloadDaemon(ctx)
}

func removeCorruptSysext(ctx context.Context, appName string) error {
	slog.WarnContext(ctx, "Unable to load application '"+appName+"' due to a corrupt on-disk image, attempting to cleanup")

	removedAtLestOneSysext := false

	// Check each on-disk sysext image, and delete any that are corrupt.
	versions, err := os.ReadDir(LocalExtensionsPath)
	if err != nil {
		return err
	}

	for _, version := range versions {
		sysextImageFile := filepath.Join(LocalExtensionsPath, version.Name(), appName+".raw")

		_, err := os.Stat(sysextImageFile)
		if err == nil {
			err := VerifyExtension(ctx, sysextImageFile)
			if err != nil {
				slog.WarnContext(ctx, "sysext image for application '"+appName+"' version "+version.Name()+" is corrupt, deleting")

				// Remove the corrupt sysext image.
				err := os.Remove(sysextImageFile)
				if err != nil {
					return err
				}

				// Opportunistically attempt to cleanup a directory that might have become empty.
				_ = os.Remove(filepath.Join(LocalExtensionsPath, version.Name()))

				removedAtLestOneSysext = true
			}
		}
	}

	if !removedAtLestOneSysext {
		return errors.New("systemd-sysext failed to load sysext image for application '" + appName + "', but all image(s) on-disk validated correctly")
	}

	slog.InfoContext(ctx, "System must reboot to finalize cleanup of corrupt application image(s), rebooting in five seconds")

	time.Sleep(5 * time.Second)

	// After deleting corrupt image(s). reboot the system. On next boot, IncusOS will detect and use any remaining on-disk
	// versions of the application. Additionally, as part of normal startup IncusOS will check for updates, which ensures
	// that even if all on-disk images were corrupt and deleted, the system will automatically download a known-good application
	// image and use that to guarantee that the system remains operational as expected.
	err = SystemReboot(ctx)
	if err != nil {
		return err
	}

	// Sleep to delay any further actions while the system is rebooting.
	time.Sleep(5 * time.Second)

	return nil
}
