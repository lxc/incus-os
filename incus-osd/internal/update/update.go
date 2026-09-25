package update

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	ocapi "github.com/FuturFusion/operations-center/shared/api"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/scheduling"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/storage"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/tui"
)

const (
	// UpdateCheckJob represents the job to check for system updates.
	UpdateCheckJob scheduling.JobName = "update_check"
)

// CheckRespectMaintenanceWindows runs a full (applications and OS) update check
// respecting any defined maintenance windows. Typically this should be called
// automatically by the daemon scheduler.
func CheckRespectMaintenanceWindows(ctx context.Context, s *state.State) error {
	// Check if we are within a defined maintenance window.
	// FIXME: This changes behavior. Prior, we would sleep min(duration, next maintenance window)
	//        but now we only run the check on a fixed schedule. This may cause IncusOS to
	//        skip maintenance windows if the duration is larger than the smallest maintenance window.
	inMaintenanceWindow := len(s.System.Update.Config.MaintenanceWindows) == 0
	for _, window := range s.System.Update.Config.MaintenanceWindows {
		if window.IsCurrentlyActive() {
			inMaintenanceWindow = true

			break
		}
	}

	if !inMaintenanceWindow {
		s.System.Update.State.Status = "Skipping update check outside of maintenance window(s)"
		slog.InfoContext(ctx, s.System.Update.State.Status)

		return nil
	}

	return Check(ctx, s)
}

// Check runs a full (applications and OS) update check regardless of any defined
// maintenance windows. Typically this should be called  as the system starts up.
func Check(ctx context.Context, s *state.State) error {
	err := CheckSecureBoot(ctx, s, false, false)
	if err != nil {
		return err
	}

	err = CheckApplications(ctx, s, nil, false, false)
	if err != nil {
		return err
	}

	err = CheckOS(ctx, s, false, false)
	if err != nil {
		return err
	}

	return nil
}

// CheckWithEmptyCache runs a full (applications and OS) update check regardless
// of any defined maintenance windows. The provider's cache is forceably cleared
// to ensure the very latest updates are available. Typically this should one be
// called in response to a user specifically requesting an update check.
func CheckWithEmptyCache(ctx context.Context, s *state.State) error {
	err := CheckSecureBoot(ctx, s, true, false)
	if err != nil {
		return err
	}

	err = CheckApplications(ctx, s, nil, true, false)
	if err != nil {
		return err
	}

	err = CheckOS(ctx, s, true, false)
	if err != nil {
		return err
	}

	return nil
}

// CheckSecureBoot checks for an available Secure Boot update.
func CheckSecureBoot(ctx context.Context, s *state.State, clearCache bool, forceApplyUpdate bool) error {
	// Only check for updates if Secure Boot is enabled.
	if s.SecureBootDisabled {
		return nil
	}

	p, t, err := updateCommonPrep(ctx, s, clearCache)
	if err != nil {
		return err
	}

	// Save when we last performed an update check.
	s.System.Update.State.LastCheck = time.Now()
	s.System.Update.State.Status = "Checking for Secure Boot updates"

	// Check for and apply any Secure Boot key updates before performing any OS or application updates.
	_, err = checkAndDownloadUpdate(ctx, s, t, p, TypeSecureBoot, "", forceApplyUpdate)
	if err != nil {
		s.System.Update.State.Status = "Failed to check for Secure Boot key updates"
		showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

		return err
	}

	dismissUpdateModal(t)

	s.System.Update.State.Status = "Secure Boot update check completed"

	return nil
}

// CheckApplications checks a specified list of applications for any available updates and will install
// any application not currently installed. If no applications are specified, all currently installed
// applications will be checked.
func CheckApplications(ctx context.Context, s *state.State, toInstall []string, clearCache bool, forceApplyUpdate bool) error {
	p, t, err := updateCommonPrep(ctx, s, clearCache)
	if err != nil {
		return err
	}

	// Save when we last performed an update check.
	s.System.Update.State.LastCheck = time.Now()
	s.System.Update.State.Status = "Checking for application updates"

	if toInstall == nil {
		// When no specific application(s) are specified, default to checking
		// for updates each currently installed application.
		toInstall, err = applications.GetInstallApplications(ctx, s)
		if err != nil {
			s.System.Update.State.Status = err.Error()
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			return err
		}
	}

	// Check for application updates.
	appsUpdated := map[string]string{}

	for _, appName := range toInstall {
		newAppVersion, err := checkAndDownloadUpdate(ctx, s, t, p, TypeApplication, appName, forceApplyUpdate)
		if err != nil {
			s.System.Update.State.Status = "Failed to check for update for application '" + appName + "'"
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			continue
		}

		if newAppVersion != "" {
			appsUpdated[appName] = newAppVersion
		}
	}

	dismissUpdateModal(t)

	// Refresh extensions and reload applications if any have been updated or freshly installed.
	if len(appsUpdated) > 0 {
		// Apply the system extensions.
		slog.DebugContext(ctx, "Refreshing system extensions")

		err := applications.RefreshExtensions(ctx, s)
		if err != nil {
			s.System.Update.State.Status = "Failed to refresh system extensions"
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			return err
		}

		// If this check isn't run during initial system startup, notify the applications that
		// they need to update/restart.
		if s.OS.SystemIsReady {
			for appName, appVersion := range appsUpdated {
				err := reloadApplication(ctx, s, appName, appVersion)
				if err != nil {
					s.System.Update.State.Status = "Failed to reload application '" + appName + "'"
					showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)
				}
			}
		}
	}

	s.System.Update.State.Status = "Application update check completed"

	return nil
}

// CheckOS checks for an available OS update.
func CheckOS(ctx context.Context, s *state.State, clearCache bool, forceApplyUpdate bool) error {
	p, t, err := updateCommonPrep(ctx, s, clearCache)
	if err != nil {
		return err
	}

	// Save when we last performed an update check.
	s.System.Update.State.LastCheck = time.Now()
	s.System.Update.State.Status = "Checking for OS updates"

	// Check for the latest OS update.
	newInstalledOSVersion, err := checkAndDownloadUpdate(ctx, s, t, p, TypeOS, "", forceApplyUpdate)
	if err != nil {
		s.System.Update.State.Status = "Failed to check for OS updates"
		showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

		return err
	}

	dismissUpdateModal(t)

	s.System.Update.State.Status = s.OS.Name + " has been updated to version " + newInstalledOSVersion

	return nil
}

func updateCommonPrep(ctx context.Context, s *state.State, clearCache bool) (providers.Provider, *tui.TUI, error) {
	p, err := providers.Load(ctx, s, false)
	if err != nil {
		return nil, nil, err
	}

	t, err := tui.GetTUI(nil)
	if err != nil {
		return nil, nil, err
	}

	if clearCache {
		err := p.ClearCache(ctx)
		if err != nil {
			s.System.Update.State.Status = "Failed to clear provider cache"
			slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

			return nil, nil, err
		}
	}

	return p, t, nil
}

// reloadApplication wraps common logic used when starting/updating an application after it is updated.
func reloadApplication(ctx context.Context, s *state.State, appName string, appVersion string) error {
	// Get the provider.
	p, err := providers.Load(ctx, s, false)
	if err != nil {
		return err
	}

	// Get the application.
	app, err := applications.Load(ctx, s, appName)
	if err != nil {
		s.System.Update.State.Status = "Failed to load application"
		showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

		return err
	}

	// Start/reload the application.
	if app.IsRunning(ctx) {
		slog.InfoContext(ctx, "Reloading application", "name", appName, "version", appVersion)

		err := app.Update(ctx)
		if err != nil {
			s.System.Update.State.Status = "Failed to reload application"
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			if app.IsPrimary() {
				slog.WarnContext(ctx, "Primary application "+app.Name()+" failed to reload; attempting to enable fallback HTTPS server for basic connectivity")

				s.RequestFallbackListener()
			}

			return err
		}
	} else {
		err := applications.StartInitialize(ctx, s, appName)
		if err != nil {
			s.System.Update.State.Status = "Failed to start application"
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			if app.IsPrimary() {
				slog.WarnContext(ctx, "Primary application "+app.Name()+" failed to start; attempting to enable fallback HTTPS server for basic connectivity")

				s.RequestFallbackListener()
			}

			return err
		}
	}

	return nil
}

// rebootSystem reboots through the daemon's regular shutdown sequence so applications and services
// are stopped in order. The trigger channel isn't available during recovery or early startup, in
// which case the system is rebooted directly.
func rebootSystem(ctx context.Context, s *state.State) error {
	if s.TriggerReboot == nil {
		return systemd.SystemReboot(ctx)
	}

	// Non-blocking send in case a reboot is already pending.
	select {
	case s.TriggerReboot <- true:
	default:
	}

	return nil
}

// checkAndDownloadUpdate performs a check for the specified update, and if found attempts to download it.
func checkAndDownloadUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, ut Type, appName string, forceApplyUpdate bool) (string, error) {
	s.UpdateMutex.Lock()
	defer s.UpdateMutex.Unlock()

	slog.DebugContext(ctx, "Checking for "+ut.String()+" updates")

	if s.System.Update.State.NeedsReboot {
		slog.DebugContext(ctx, "A reboot of the system is required to finalize a pending update")

		if ut == TypeSecureBoot {
			return "", nil
		}
	}

	// Get the appropriate update.
	var update providers.CommonUpdate

	var err error

	switch ut {
	case TypeSecureBoot:
		update, err = p.GetSecureBootCertUpdate(ctx)
	case TypeOS:
		update, err = p.GetOSUpdate(ctx)
	case TypeApplication:
		update, err = p.GetApplicationUpdate(ctx, appName)
	default:
		return "", errors.New("unrecognized update type '" + ut.String() + "'")
	}

	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			slog.DebugContext(ctx, ut.String()+" update provider doesn't currently have any update", "channel", s.System.Update.Config.Channel)

			return "", nil
		}

		return "", err
	}

	updateNeeded := false

	// Skip any update that isn't newer than what we are already running.
	switch ut {
	case TypeSecureBoot:
		updateNeeded = update.Version() != s.SecureBoot.Version

		if updateNeeded && s.SecureBoot.Version != "" && s.SecureBoot.Version != update.Version() && !update.IsNewerThan(s.SecureBoot.Version) {
			return "", errors.New("installed Secure Boot keys version (" + s.SecureBoot.Version + ") is newer than available update (" + update.Version() + "); skipping")
		}
	case TypeOS:
		// If we're running from the backup image don't attempt to re-update to a broken version.
		if !s.System.Update.State.NeedsReboot && s.OS.RunningFromBackup() && s.OS.NextRelease == update.Version() {
			slog.WarnContext(ctx, "Latest "+s.OS.Name+" image version "+s.OS.NextRelease+" has been identified as problematic, skipping update")

			return "", nil
		}

		updateNeeded = update.Version() != s.OS.RunningRelease && update.Version() != s.OS.NextRelease

		if updateNeeded && s.OS.RunningRelease != update.Version() && !update.IsNewerThan(s.OS.RunningRelease) {
			return "", errors.New("local " + s.OS.Name + " version (" + s.OS.RunningRelease + ") is newer than available update (" + update.Version() + "); skipping")
		}
	case TypeApplication:
		app, err := applications.Load(ctx, s, appName)
		if err != nil {
			return "", err
		}

		updateNeeded = update.Version() != app.Version()

		if updateNeeded && app.Version() != "" && !update.IsNewerThan(app.Version()) {
			return "", errors.New("local application " + appName + " version (" + app.Version() + ") is newer than available update (" + update.Version() + "); skipping")
		}
	default:
		// An invalid update type has been handled previously.
	}

	// Apply the update.
	if updateNeeded || forceApplyUpdate {
		// Before applying the update, check current disk space.
		err := storage.CheckMinimumDiskSpace(ctx, "/")
		if err != nil {
			return "", err
		}

		return applyUpdate(ctx, s, t, update, appName)
	} else if !s.OS.SystemIsReady {
		if ut == TypeApplication {
			slog.DebugContext(ctx, "System is already running latest application version", "application", appName, "channel", s.System.Update.Config.Channel, "version", update.Version())
		} else {
			slog.DebugContext(ctx, "System is already running latest "+ut.String()+" version", "channel", s.System.Update.Config.Channel, "version", update.Version())
		}
	}

	return "", nil
}

func applyUpdate(ctx context.Context, s *state.State, t *tui.TUI, update providers.CommonUpdate, appName string) (string, error) {
	updateModal := t.GetModal("update")

	if t.GetModal("update") == nil {
		updateModal = t.AddModal(s.OS.Name+" Update", "update")
	}

	var targetPath string

	switch update.(type) {
	case providers.SecureBootCertUpdate:
		targetPath = "/tmp/"

		slog.InfoContext(ctx, "Downloading SecureBoot update", "channel", s.System.Update.Config.Channel, "version", update.Version())
		updateModal.Update("Downloading SecureBoot update " + update.Version() + " from channel " + s.System.Update.Config.Channel)
	case providers.OSUpdate:
		targetPath = systemd.SystemUpdatesPath

		slog.InfoContext(ctx, "Downloading OS update", "channel", s.System.Update.Config.Channel, "version", update.Version())
		updateModal.Update("Downloading OS update " + update.Version() + " from channel " + s.System.Update.Config.Channel)
	case providers.ApplicationUpdate:
		targetPath = filepath.Join(systemd.LocalExtensionsPath, update.Version())

		slog.InfoContext(ctx, "Downloading application update", "application", appName, "channel", s.System.Update.Config.Channel, "version", update.Version())
		updateModal.Update("Downloading application update " + appName + " version " + update.Version() + " from channel " + s.System.Update.Config.Channel)
	default:
		// An invalid update type has been handled previously in checkDownloadUpdate().
	}

	// Download the update.
	err := update.Download(ctx, targetPath, updateModal.UpdateProgress)
	if err != nil {
		updateModal.Done()

		return "", err
	}

	// Hide the progress bar.
	updateModal.UpdateProgress(0.0)

	switch u := update.(type) {
	case providers.SecureBootCertUpdate:
		slog.InfoContext(ctx, "Applying Secure Boot certificate update", "version", update.Version())
		updateModal.Update("Applying Secure Boot certificate update version " + update.Version())

		// Immediately set FullyApplied to false and save state to disk.
		s.SecureBoot.FullyApplied = false
		_ = s.Save()

		needsReboot, err := secureboot.UpdateSecureBootCerts(ctx, filepath.Join(targetPath, u.GetFilename()))
		if err != nil {
			return "", err
		}

		// If an EFI variable was updated, we'll either be rebooting automatically or waiting
		// for the user to restart the system before going any further.
		if needsReboot {
			updateModal.Done()

			s.System.Update.State.NeedsReboot = true

			if !s.OS.SystemIsReady {
				sbModal := t.GetModal("secureboot-update")
				if sbModal == nil {
					sbModal = t.AddModal(s.OS.Name+" SecureBoot Certificate Update", "secureboot-update")
				}

				slog.InfoContext(ctx, "Automatically rebooting system in five seconds")
				sbModal.Update("Automatically rebooting system in five seconds")

				time.Sleep(5 * time.Second)

				_ = rebootSystem(ctx, s)

				time.Sleep(60 * time.Second) // Prevent further system start up in the half second or so before things reboot.
			} else {
				// The pending reboot is indicated through the TUI header.
				slog.InfoContext(ctx, "A reboot is required to finalize the update")
			}

			return "", nil
		}

		// Update state once all SecureBoot keys are updated.
		s.SecureBoot.Version = update.Version()
		s.SecureBoot.FullyApplied = true
	case providers.OSUpdate:
		// Apply the update and reboot if first time through loop, otherwise wait for user to reboot system.
		slog.InfoContext(ctx, "Applying OS update", "version", update.Version())
		updateModal.Update("Applying " + s.OS.Name + " update version " + update.Version())

		err = systemd.ApplySystemUpdate(ctx, update.Version())
		if err != nil {
			return "", err
		}

		// Record the new release.
		if !s.System.Update.Config.AutoReboot && s.OS.SystemIsReady {
			// Mark the system as needing a reboot down the line.
			s.System.Update.State.NeedsReboot = true
		}

		s.OS.NextRelease = update.Version()
		_ = s.Save()

		// Record the state of auto-unlocked LUKS devices. With some TPMs this can be slow, so cache the
		// result after applying an OS update rather than needing to determine it each time a request
		// arrives via the API.
		s.System.Security.State.EncryptedVolumes, err = systemd.ListEncryptedVolumes(ctx)
		if err != nil {
			return "", err
		}

		// Notify the provider.
		err = providers.Notify(ctx, s, ocapi.ServerSelfUpdateCauseOSUpdateApplied)
		if err != nil {
			return "", err
		}

		// Handle reboot if needed.
		if s.System.Update.Config.AutoReboot || !s.OS.SystemIsReady {
			// The reboot handler notifies the provider itself when going through the regular shutdown sequence.
			if s.TriggerReboot == nil {
				err := providers.Notify(ctx, s, ocapi.ServerSelfUpdateCauseSystemRebootTriggered)
				if err != nil {
					return "", err
				}
			}

			err = rebootSystem(ctx, s)
			if err != nil {
				return "", err
			}

			// Wait 10s to allow time for the system to reboot.
			time.Sleep(10 * time.Second)
		}

	case providers.ApplicationUpdate:
		// Verify the application is signed with a trusted key in the kernel's keyring.
		err = systemd.VerifyExtension(ctx, filepath.Join(targetPath, appName+".raw"))
		if err != nil {
			return "", err
		}

		// Ensure a proper application symlink exists.
		_, err = os.Lstat(filepath.Join(systemd.SystemExtensionsPath, appName+".raw"))
		if err != nil && os.IsNotExist(err) {
			// Ensure /var/lib/extensions/ exists.
			err := os.MkdirAll(systemd.SystemExtensionsPath, 0o700)
			if err != nil {
				return "", err
			}

			// Create the application symlink.
			err = os.Symlink(filepath.Join(targetPath, appName+".raw"), filepath.Join(systemd.SystemExtensionsPath, appName+".raw"))
			if err != nil {
				return "", err
			}
		}

		// Load the application
		app, err := applications.Load(ctx, s, appName)
		if err != nil {
			return "", err
		}

		// If we're updating an existing application and are running from the backup IncusOS
		// image, after verifying the new application sysext don't automatically update to it.
		if app.IsInstalled() && !s.System.Update.State.NeedsReboot && s.OS.RunningFromBackup() {
			slog.WarnContext(ctx, "Successfully downloaded application update, but not auto-updating while running from backup image", "application", appName)

			// Add the newer version to list of available versions.
			av := app.AvailableVersions()
			av = append(av, update.Version())
			app.SetVersions(app.Version(), av)
		} else {
			// Record newly installed application and save state to disk.
			app.SetVersions(update.Version(), nil)

			// Notify the provider.
			err = providers.Notify(ctx, s, ocapi.ServerSelfUpdateCauseApplicationUpdateApplied)
			if err != nil {
				return "", err
			}
		}
	default:
		// An invalid update type has been handled previously in checkDownloadUpdate().
	}

	return update.Version(), nil
}

func dismissUpdateModal(t *tui.TUI) {
	updateModal := t.GetModal("update")
	if updateModal != nil {
		updateModal.Done()
	}
}

func showModalError(ctx context.Context, osName string, msg string, err error, p providers.Provider) {
	slog.ErrorContext(ctx, msg, "err", err.Error(), "provider", p.Type())

	t, tuiErr := tui.GetTUI(nil)
	if tuiErr != nil {
		return
	}

	updateModal := t.GetModal("update")

	if updateModal == nil {
		updateModal = t.AddModal(osName+" Update", "update")
	}

	updateModal.Update("[red]Error[white] " + msg + ": " + err.Error() + " (provider: " + p.Type() + ")")
}
