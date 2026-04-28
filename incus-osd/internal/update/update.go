package update

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/lxc/incus-os/incus-osd/internal/applications"
	"github.com/lxc/incus-os/incus-osd/internal/providers"
	"github.com/lxc/incus-os/incus-osd/internal/secureboot"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
	"github.com/lxc/incus-os/incus-osd/internal/tui"
)

// Checker utilizes the given provider to check for Secure Boot, OS, and application updates.
func Checker(ctx context.Context, s *state.State, p providers.Provider, isStartupCheck bool, isUserRequested bool) { //nolint:revive
	t, err := tui.GetTUI(nil)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get TUI application: "+err.Error())

		return
	}

	for {
		// Determine if a primary application is installed or not.
		primaryApplication, err := applications.GetPrimary(ctx, s, false)
		if err != nil && !errors.Is(err, applications.ErrNoPrimary) {
			s.System.Update.State.Status = "Failed to check if a primary application is installed"
			slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

			break
		}

		// If updates are disabled, skip for an hour.
		if !isUserRequested && s.System.Update.Config.CheckFrequency == "never" {
			// Only respect the request to disable update checks if a primary application is installed.
			// If not, we can find ourselves in a situation where IncusOS boots but is inaccessible
			// because no primary application is running, and IncusOS would never attempt to install one.
			if primaryApplication != nil {
				if isStartupCheck {
					break
				}

				time.Sleep(time.Hour)

				continue
			}
		}

		// Sleep at the top of each loop, except if we're performing a startup or manual check.
		if !isStartupCheck && !isUserRequested {
			timeSinceCheck := time.Since(s.System.Update.State.LastCheck)
			rawFrequency := s.System.Update.Config.CheckFrequency

			// If no primary application is installed and the check frequency is never, override
			// that value to six hours. Once a primary application is successfully installed,
			// we will honor the request to disable update checks.
			if primaryApplication == nil && rawFrequency == "never" {
				rawFrequency = "6h"
			}

			frequency, err := time.ParseDuration(rawFrequency)
			if err != nil {
				// Shouldn't be possible, we validate on update.
				s.System.Update.State.Status = "Failed to parse update frequency"
				slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

				break
			}

			if frequency < 0 {
				// Shouldn't be possible, we validate on update.
				s.System.Update.State.Status = "Update frequency must be a positive value"
				slog.ErrorContext(ctx, s.System.Update.State.Status, "err", "Update frequency must be a positive value")

				break
			}

			// If any maintenance windows are defined, limit the time to sleep to be a minimum
			// of the configured check frequency and the start of the next maintenance window,
			// whichever is shorter.
			for _, window := range s.System.Update.Config.MaintenanceWindows {
				if window.TimeUntilActive() > 0 && window.TimeUntilActive() < frequency {
					frequency = window.TimeUntilActive()
				}
			}

			if timeSinceCheck < frequency {
				// Add one minute to the calculated sleep to protect against an edge case
				// where we try to do an update check right at the start of a maintenance window.
				time.Sleep(frequency - timeSinceCheck + 1*time.Minute)
			}
		}

		// Save when we last performed an update check.
		s.System.Update.State.LastCheck = time.Now()
		s.System.Update.State.Status = "Running update check"

		// Check maintenance window, except if we're performing a startup or manual check.
		if !isStartupCheck && !isUserRequested {
			// Check that we are within a defined maintenance window.
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

				continue
			}
		}

		// If user requested, clear cache.
		if isUserRequested {
			err := p.ClearCache(ctx)
			if err != nil {
				s.System.Update.State.Status = "Failed to clear provider cache"
				slog.ErrorContext(ctx, s.System.Update.State.Status, "err", err.Error())

				break
			}
		}

		// Check for and apply any Secure Boot key updates before performing any OS or application updates.
		// Only check if Secure Boot is enabled.
		if !s.SecureBootDisabled {
			_, err := CheckAndDownloadUpdate(ctx, s, t, p, TypeSecureBoot, "", isStartupCheck)
			if err != nil {
				s.System.Update.State.Status = "Failed to check for Secure Boot key updates"
				showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}
		}

		// Determine what applications to install.
		toInstall, err := applications.GetInstallApplications(ctx, s)
		if err != nil {
			s.System.Update.State.Status = err.Error()
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Check for application updates.
		appsUpdated := map[string]string{}

		for _, appName := range toInstall {
			newAppVersion, err := CheckAndDownloadUpdate(ctx, s, t, p, TypeApplication, appName, isStartupCheck)
			if err != nil {
				s.System.Update.State.Status = "Failed to check for application updates"
				showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

				break
			}

			if newAppVersion != "" {
				appsUpdated[appName] = newAppVersion
			}
		}

		// Apply the system extensions.
		if len(appsUpdated) > 0 {
			slog.DebugContext(ctx, "Refreshing system extensions")

			err := applications.RefreshExtensions(ctx, s)
			if err != nil {
				s.System.Update.State.Status = "Failed to refresh system extensions"
				showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}
		}

		// Check for the latest OS update.
		newInstalledOSVersion, err := CheckAndDownloadUpdate(ctx, s, t, p, TypeOS, "", isStartupCheck)
		if err != nil {
			s.System.Update.State.Status = "Failed to check for OS updates"
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Notify the applications that they need to update/restart.
		if !isStartupCheck {
			for appName, appVersion := range appsUpdated {
				_ = reloadApplication(ctx, s, appName, appVersion)
			}
		}

		HandlePostUpdateMessage(s, t, newInstalledOSVersion)

		if isStartupCheck || isUserRequested {
			// If running a one-time update, we're done.
			break
		}
	}
}

// InstallUpdateApp wraps common logic used when manually installing or updating an application.
func InstallUpdateApp(ctx context.Context, s *state.State, appName string, clearCache bool) error {
	// Get the TUI.
	t, err := tui.GetTUI(nil)
	if err != nil {
		return err
	}

	// Get the provider.
	p, err := providers.Load(ctx, s)
	if err != nil {
		return err
	}

	if clearCache {
		// Clear the provider cache to get the latest available version for an update.
		err := p.ClearCache(ctx)
		if err != nil {
			return err
		}
	}

	// Attempt to download the application.
	newAppVersion, err := CheckAndDownloadUpdate(ctx, s, t, p, TypeApplication, appName, false)
	if err != nil {
		return err
	}

	// If the application was freshly installed or updated, refresh the sysext images and trigger the application's update method.
	if newAppVersion != "" {
		// Display a post-update message.
		HandlePostUpdateMessage(s, t, "")

		err := applications.RefreshExtensions(ctx, s)
		if err != nil {
			return err
		}

		err = reloadApplication(ctx, s, appName, newAppVersion)
		if err != nil {
			return err
		}
	}

	return nil
}

// reloadApplication wraps common logic used when starting/updating an application after it is updated.
func reloadApplication(ctx context.Context, s *state.State, appName string, appVersion string) error {
	// Get the provider.
	p, err := providers.Load(ctx, s)
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

			return err
		}
	} else {
		err := applications.StartInitialize(ctx, s, appName)
		if err != nil {
			s.System.Update.State.Status = "Failed to start application"
			showModalError(ctx, s.OS.Name, s.System.Update.State.Status, err, p)

			return err
		}
	}

	return nil
}

// HandlePostUpdateMessage takes care of displaying either a reboot message if needed, or ensuring
// that the update modal is dismissed.
func HandlePostUpdateMessage(s *state.State, t *tui.TUI, osVersion string) {
	updateModal := t.GetModal("update")

	if osVersion != "" {
		if updateModal == nil {
			updateModal = t.AddModal(s.OS.Name+" Update", "update")
		}

		s.System.Update.State.Status = s.OS.Name + " has been updated to version " + osVersion
		updateModal.Update(s.OS.Name + " has been updated to version " + osVersion + ".\nPlease reboot the system to finalize update.")
	} else {
		s.System.Update.State.Status = "Update check completed"

		if updateModal != nil {
			updateModal.Done()
		}
	}
}

// CheckAndDownloadUpdate performs a check for the specified update, and if found attempts to download it.
func CheckAndDownloadUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, ut Type, appName string, isStartupCheck bool) (string, error) {
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
			slog.DebugContext(ctx, ut.String()+" update provider doesn't currently have any update")

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
	if updateNeeded {
		return applyUpdate(ctx, s, t, update, appName, isStartupCheck)
	} else if isStartupCheck {
		if ut == TypeApplication {
			slog.DebugContext(ctx, "System is already running latest application version", "application", appName, "version", update.Version())
		} else {
			slog.DebugContext(ctx, "System is already running latest "+ut.String()+" version", "version", update.Version())
		}
	}

	return "", nil
}

func applyUpdate(ctx context.Context, s *state.State, t *tui.TUI, update providers.CommonUpdate, appName string, isStartupCheck bool) (string, error) {
	updateModal := t.GetModal("update")

	if t.GetModal("update") == nil {
		updateModal = t.AddModal(s.OS.Name+" Update", "update")
	}

	var targetPath string

	switch update.(type) {
	case providers.SecureBootCertUpdate:
		targetPath = "/tmp/"

		slog.InfoContext(ctx, "Downloading SecureBoot update", "version", update.Version())
		updateModal.Update("Downloading SecureBoot update " + update.Version())
	case providers.OSUpdate:
		targetPath = systemd.SystemUpdatesPath

		slog.InfoContext(ctx, "Downloading OS update", "version", update.Version())
		updateModal.Update("Downloading OS update " + update.Version())
	case providers.ApplicationUpdate:
		targetPath = filepath.Join(systemd.LocalExtensionsPath, update.Version())

		slog.InfoContext(ctx, "Downloading application update", "application", appName, "version", update.Version())
		updateModal.Update("Downloading application update " + appName + " update " + update.Version())
	default:
		// An invalid update type has been handled previously in checkDownloadUpdate().
	}

	// Download the update.
	err := update.Download(ctx, targetPath, updateModal.UpdateProgress)
	if err != nil {
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

			sbModal := t.GetModal("secureboot-update")

			if t.GetModal("secureboot-update") == nil {
				sbModal = t.AddModal(s.OS.Name+" SecureBoot Certificate Update", "secureboot-update")
			}

			if isStartupCheck {
				slog.InfoContext(ctx, "Automatically rebooting system in five seconds")
				sbModal.Update("Automatically rebooting system in five seconds")

				time.Sleep(5 * time.Second)

				_ = systemd.SystemReboot(ctx)

				time.Sleep(60 * time.Second) // Prevent further system start up in the half second or so before things reboot.
			} else {
				slog.InfoContext(ctx, "A reboot is required to finalize the update")
				sbModal.Update("A reboot is required to finalize the update")
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
		if !s.System.Update.Config.AutoReboot && !isStartupCheck {
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

		// Handle reboot if needed.
		if s.System.Update.Config.AutoReboot || isStartupCheck {
			// Rather than closing s.TriggerReboot, explicitly reboot here. This is needed when
			// applying an update via the recovery mechanism since at that point in startup
			// the channels won't be available yet.
			err := systemd.SystemReboot(ctx)
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

		// Load the application
		app, err := applications.Load(ctx, s, appName)
		if err != nil {
			return "", err
		}

		// If we're updating an existing application and are running from the backup IncusOS
		// image, after verifying the new application sysext don't automatically update to it.
		if app.IsInstalled() && s.OS.RunningFromBackup() {
			slog.WarnContext(ctx, "Successfully downloaded application update, but not auto-updating while running from backup image", "application", appName)

			// Add the newer version to list of available versions.
			av := app.AvailableVersions()
			av = append(av, update.Version())
			app.SetVersions(app.Version(), av)
		} else {
			// Record newly installed application and save state to disk.
			app.SetVersions(update.Version(), nil)
		}
	default:
		// An invalid update type has been handled previously in checkDownloadUpdate().
	}

	return update.Version(), nil
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
