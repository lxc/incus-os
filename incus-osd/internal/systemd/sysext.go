package systemd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/util"
)

type availableApplicationVersions struct {
	appVersion        string // The version of the application according to the current state.
	currentOSVersion  string // The version of the currently running IncusOS.
	otherOSVersion    string // The version of the other IncusOS UKI, if any.
	latestDiskVersion string // The latest version of the application that exists on disk.
}

var priorBootRelease string

// RefreshExtensions causes systemd-sysext to re-scan and reload the system extensions.
func RefreshExtensions(ctx context.Context, currentApps map[string]api.Application, osInfo *state.OS) error {
	// If no applications are defined, there's nothing to do.
	if len(currentApps) == 0 {
		return nil
	}

	// Begin by collecting information about the expected version(s) that exist
	// on-disk for each installed application.
	appVersions, err := getApplicationsVersions(currentApps)
	if err != nil {
		return err
	}

	// Get the prior booted release.
	if priorBootRelease == "" {
		priorBootRelease = util.PriorBootRelease(ctx)
	}

	// Ensure /var/lib/extensions/ exists.
	err = os.MkdirAll(SystemExtensionsPath, 0o700)
	if err != nil {
		return err
	}

	// Remove any existing symlinked sysext images.
	files, err := os.ReadDir(SystemExtensionsPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		// Important! We only remove symlinks. This ensures if we somehow end up
		// in a situation where only an unversioned sysext image exists under
		// /var/lib/extensions/ we don't wipe it which could potentially brick
		// IncusOS if that image happened to be a primary application.
		if file.Type()&fs.ModeSymlink != 0 {
			err := os.Remove(filepath.Join(SystemExtensionsPath, file.Name()))
			if err != nil {
				return err
			}
		}
	}

	// Symlink the best version of each application and ensure the application state
	// reflects the selected on-disk version.
	for name, version := range appVersions {
		bestVersion := bestApplicationVersion(name, version, priorBootRelease, osInfo.RunningRelease)

		// Create the new symlink.
		err := os.Symlink(filepath.Join(LocalExtensionsPath, bestVersion, name+".raw"), filepath.Join(SystemExtensionsPath, name+".raw"))
		if err != nil {
			return err
		}

		// Update the application's state to reflect the on-disk version.
		newAppState := currentApps[name]
		newAppState.State.Version = bestVersion

		newAppState.State.AvailableVersions = []string{version.appVersion}

		if version.currentOSVersion != "" {
			newAppState.State.AvailableVersions = append(newAppState.State.AvailableVersions, version.currentOSVersion)
		}

		if version.latestDiskVersion != "" {
			newAppState.State.AvailableVersions = append(newAppState.State.AvailableVersions, version.latestDiskVersion)
		}

		if version.otherOSVersion != "" {
			newAppState.State.AvailableVersions = append(newAppState.State.AvailableVersions, version.otherOSVersion)
		}

		slices.Sort(newAppState.State.AvailableVersions)
		newAppState.State.AvailableVersions = slices.Compact(newAppState.State.AvailableVersions)

		currentApps[name] = newAppState
	}

	// Reload the extensions.
	err = reloadExtensions(ctx)
	if err != nil {
		return err
	}

	// Update priorBootRelease. This ensures subsequent calls to RefreshExtensions() behave as expected
	// and don't unexpectedly try to force-reset application versions.
	priorBootRelease = osInfo.RunningRelease

	// Remove any old versions of each application.
	for appName, appInfo := range currentApps {
		err := removeOldAppVersions(appName, appInfo.State.AvailableVersions)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveExtension removes all versions of the specified system extension image from disk.
func RemoveExtension(ctx context.Context, name string) error {
	// Remove symlink from /var/lib/extensions/.
	err := os.Remove(filepath.Join(SystemExtensionsPath, name+".raw"))
	if err != nil {
		return err
	}

	// Remove all versioned sysext images.
	err = removeOldAppVersions(name, nil)
	if err != nil {
		return err
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

func getApplicationsVersions(currentApps map[string]api.Application) (map[string]*availableApplicationVersions, error) {
	appVersions := make(map[string]*availableApplicationVersions)

	ukiVersions, err := util.GetUKIVersions()
	if err != nil {
		return appVersions, err
	}

	for name, app := range currentApps {
		currentOSVersion := ukiVersions.CurrentVersion

		_, err := os.Stat(filepath.Join(LocalExtensionsPath, currentOSVersion, name+".raw"))
		if err != nil {
			currentOSVersion = ""
		}

		otherOSVersion := ukiVersions.OtherVersion

		_, err = os.Stat(filepath.Join(LocalExtensionsPath, otherOSVersion, name+".raw"))
		if err != nil {
			otherOSVersion = ""
		}

		appVersions[name] = &availableApplicationVersions{
			appVersion:        app.State.Version,
			currentOSVersion:  currentOSVersion,
			otherOSVersion:    otherOSVersion,
			latestDiskVersion: "", // Populated below
		}
	}

	// Get information about each available sysext image to populate the latest disk version
	// field for each application.
	dirEntries, err := os.ReadDir(LocalExtensionsPath)
	if err != nil {
		return appVersions, err
	}

	for _, entry := range dirEntries {
		// Only consider version directories.
		if !entry.IsDir() {
			continue
		}

		// For each application, check if it exists on disk with the given version,
		// and if so check if it's a newer version than we currently have recorded.
		for appName := range appVersions {
			_, err := os.Stat(filepath.Join(LocalExtensionsPath, entry.Name(), appName+".raw"))
			if err == nil {
				if appVersions[appName].latestDiskVersion == "" || strings.Compare(appVersions[appName].latestDiskVersion, entry.Name()) < 0 {
					appVersions[appName].latestDiskVersion = entry.Name()
				}
			}
		}
	}

	// Ensure there's at least one version of each application on disk.
	for name, version := range appVersions {
		if version.latestDiskVersion == "" {
			return appVersions, errors.New("no on-disk sysext image found for application '" + name + "'")
		}
	}

	return appVersions, nil
}

func bestApplicationVersion(appName string, version *availableApplicationVersions, priorRelease string, runningRelease string) string {
	// For now, return the current application version.
	return version.appVersion
}

func removeOldAppVersions(appName string, skipVersions []string) error {
	dirEntries, err := os.ReadDir(LocalExtensionsPath)
	if err != nil {
		return err
	}

	// Iterate through each directory under /var/lib/incus-os-extensions/, which
	// corresponds to the version of one or more installed applications.
	for _, entry := range dirEntries {
		// Only consider version directories.
		if !entry.IsDir() {
			continue
		}

		// Don't remove any version that is in the skipVersions list.
		if slices.Contains(skipVersions, entry.Name()) {
			continue
		}

		// Check if an application image exists, and if so, remove it.
		_, err := os.Stat(filepath.Join(LocalExtensionsPath, entry.Name(), appName+".raw"))
		if err == nil {
			err := os.Remove(filepath.Join(LocalExtensionsPath, entry.Name(), appName+".raw"))
			if err != nil {
				return err
			}

			// Opportunistically attempt to remove the directory. This will fail
			// if it is non-empty, which is OK.
			_ = os.Remove(filepath.Join(LocalExtensionsPath, entry.Name()))
		}
	}

	return nil
}
