package applications

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
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
func RefreshExtensions(ctx context.Context, s *state.State) error {
	// Get currently installed applications.
	apps, err := GetInstalled(ctx, s)
	if err != nil {
		return err
	}

	// If no applications are defined, there's nothing to do.
	if len(apps) == 0 {
		return nil
	}

	// Begin by collecting information about the expected version(s) that exist
	// on-disk for each installed application.
	appVersions, err := getApplicationsVersions(apps)
	if err != nil {
		return err
	}

	// Get the prior booted release.
	if priorBootRelease == "" {
		priorBootRelease = util.PriorBootRelease(ctx)
	}

	// Ensure /var/lib/extensions/ exists.
	err = os.MkdirAll(systemd.SystemExtensionsPath, 0o700)
	if err != nil {
		return err
	}

	// Remove any existing symlinked sysext images.
	files, err := os.ReadDir(systemd.SystemExtensionsPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		// Important! We only remove symlinks. This ensures if we somehow end up
		// in a situation where only an unversioned sysext image exists under
		// /var/lib/extensions/ we don't wipe it which could potentially brick
		// IncusOS if that image happened to be a primary application.
		if file.Type()&fs.ModeSymlink != 0 {
			err := os.Remove(filepath.Join(systemd.SystemExtensionsPath, file.Name()))
			if err != nil {
				return err
			}
		}
	}

	// Symlink the best version of each application and ensure the application state
	// reflects the selected on-disk version.
	for app, version := range appVersions {
		bestVersion := bestApplicationVersion(app.Name(), version, priorBootRelease, s.OS.RunningRelease)

		// Create the new symlink.
		err := os.Symlink(filepath.Join(systemd.LocalExtensionsPath, bestVersion, app.Name()+".raw"), filepath.Join(systemd.SystemExtensionsPath, app.Name()+".raw"))
		if err != nil {
			return err
		}

		// Update the application's state to reflect the on-disk version.
		availableVersions := []string{version.appVersion}

		if version.currentOSVersion != "" {
			availableVersions = append(availableVersions, version.currentOSVersion)
		}

		if version.latestDiskVersion != "" {
			availableVersions = append(availableVersions, version.latestDiskVersion)
		}

		if version.otherOSVersion != "" {
			availableVersions = append(availableVersions, version.otherOSVersion)
		}

		slices.Sort(availableVersions)
		availableVersions = slices.Compact(availableVersions)

		app.SetVersions(bestVersion, availableVersions)
	}

	// Reload the extensions.
	err = systemd.RefreshExtensions(ctx)
	if err != nil {
		return err
	}

	// Update priorBootRelease. This ensures subsequent calls to RefreshExtensions() behave as expected
	// and don't unexpectedly try to force-reset application versions.
	priorBootRelease = s.OS.RunningRelease

	// Remove any old versions of each application.
	for _, app := range apps {
		err := removeStaleSysextImages(app.Name(), app.AvailableVersions())
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveExtension removes all versions of the specified system extension image from disk.
func RemoveExtension(ctx context.Context, app Application) error {
	// Remove symlink from /var/lib/extensions/.
	err := os.Remove(filepath.Join(systemd.SystemExtensionsPath, app.Name()+".raw"))
	if err != nil {
		return err
	}

	// Remove all versioned sysext images.
	err = removeStaleSysextImages(app.Name(), nil)
	if err != nil {
		return err
	}

	// Reload the extensions.
	return systemd.RefreshExtensions(ctx)
}

func getApplicationsVersions(apps []Application) (map[Application]*availableApplicationVersions, error) {
	appVersions := make(map[Application]*availableApplicationVersions)

	ukiVersions, err := util.GetUKIVersions()
	if err != nil {
		return appVersions, err
	}

	for _, app := range apps {
		currentOSVersion := ukiVersions.CurrentVersion

		_, err := os.Stat(filepath.Join(systemd.LocalExtensionsPath, currentOSVersion, app.Name()+".raw"))
		if err != nil {
			currentOSVersion = ""
		}

		otherOSVersion := ukiVersions.OtherVersion

		_, err = os.Stat(filepath.Join(systemd.LocalExtensionsPath, otherOSVersion, app.Name()+".raw"))
		if err != nil {
			otherOSVersion = ""
		}

		appVersions[app] = &availableApplicationVersions{
			appVersion:        app.Version(),
			currentOSVersion:  currentOSVersion,
			otherOSVersion:    otherOSVersion,
			latestDiskVersion: "", // Populated below
		}
	}

	// Get information about each available sysext image to populate the latest disk version
	// field for each application.
	dirEntries, err := os.ReadDir(systemd.LocalExtensionsPath)
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
		for app := range appVersions {
			_, err := os.Stat(filepath.Join(systemd.LocalExtensionsPath, entry.Name(), app.Name()+".raw"))
			if err == nil {
				if appVersions[app].latestDiskVersion == "" || strings.Compare(appVersions[app].latestDiskVersion, entry.Name()) < 0 {
					appVersions[app].latestDiskVersion = entry.Name()
				}
			}
		}
	}

	// Ensure there's at least one version of each application on disk. If the application version from state
	// doesn't actually exist on disk, force-set it to the most recent version available.
	for app, version := range appVersions {
		if version.latestDiskVersion == "" {
			return appVersions, errors.New("no on-disk sysext image found for application '" + app.Name() + "'")
		}

		_, err := os.Stat(filepath.Join(systemd.LocalExtensionsPath, version.appVersion, app.Name()+".raw"))
		if err != nil {
			version.appVersion = version.latestDiskVersion
		}
	}

	return appVersions, nil
}

func bestApplicationVersion(appName string, version *availableApplicationVersions, priorRelease string, runningRelease string) string {
	// If the prior boot release is unset or identical to the current running release, prefer
	// the current application version, then current OS version, then latest disk version when
	// determining what application version to return.
	if priorRelease == "" || priorRelease == runningRelease {
		// If the current application version exists on disk, return that.
		if sysextImageExists(appName, version.appVersion) {
			return version.appVersion
		}

		// If the version matching the current IncusOS version exists on disk, return that.
		if version.currentOSVersion != "" && sysextImageExists(appName, version.currentOSVersion) {
			return version.currentOSVersion
		}

		// Fallback to the latest version available on disk.
		return version.latestDiskVersion
	}

	// If the prior boot release is greater than the current running release, we've rebooted
	// into the backup A/B side. Attempt to reset the application version to match that of the
	// backup IncusOS version, then the current application version, then latest disk version.
	if strings.Compare(priorRelease, runningRelease) > 0 {
		// If the version matching the backup version exists on disk, return that.
		if version.currentOSVersion != "" && sysextImageExists(appName, version.currentOSVersion) {
			return version.currentOSVersion
		}

		// If the current application version exists on disk, return that.
		if sysextImageExists(appName, version.appVersion) {
			return version.appVersion
		}

		// Fallback to the latest version available on disk.
		return version.latestDiskVersion
	}

	// Finally, if the prior boot release is less than the current running release, we've
	// rebooted into a newer version of IncusOS. This might be a legitimate upgrade, or we're
	// booting back to the current IncusOS version from the backup A/B side. In either case,
	// set the application version to be the latest disk version. This ensures we grab the
	// latest application version, even if the underlying IncusOS system is still on an older
	// version.
	return version.latestDiskVersion
}

func sysextImageExists(name string, version string) bool {
	_, err := os.Stat(filepath.Join(systemd.LocalExtensionsPath, version, name+".raw"))

	return err == nil
}

func removeStaleSysextImages(appName string, skipVersions []string) error {
	dirEntries, err := os.ReadDir(systemd.LocalExtensionsPath)
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
		_, err := os.Stat(filepath.Join(systemd.LocalExtensionsPath, entry.Name(), appName+".raw"))
		if err == nil {
			err := os.Remove(filepath.Join(systemd.LocalExtensionsPath, entry.Name(), appName+".raw"))
			if err != nil {
				return err
			}

			// Opportunistically attempt to remove the directory. This will fail
			// if it is non-empty, which is OK.
			_ = os.Remove(filepath.Join(systemd.LocalExtensionsPath, entry.Name()))
		}
	}

	return nil
}
