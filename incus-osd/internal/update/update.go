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

	showModalError := func(msg string, err error) {
		slog.ErrorContext(ctx, msg, "err", err.Error(), "provider", p.Type())

		updateModal := t.GetModal("update")

		if t.GetModal("update") == nil {
			updateModal = t.AddModal(s.OS.Name+" Update", "update")
		}

		updateModal.Update("[red]Error[white] " + msg + ": " + err.Error() + " (provider: " + p.Type() + ")")
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
			_, err := checkDownloadUpdate(ctx, s, t, p, "SecureBoot", "", isStartupCheck)
			if err != nil {
				s.System.Update.State.Status = "Failed to check for Secure Boot key updates"
				showModalError(s.System.Update.State.Status, err)

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
			showModalError(s.System.Update.State.Status, err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Check for application updates.
		appsUpdated := map[string]string{}

		for _, appName := range toInstall {
			newAppVersion, err := checkDownloadUpdate(ctx, s, t, p, "application", appName, isStartupCheck)
			if err != nil {
				s.System.Update.State.Status = "Failed to check for application updates"
				showModalError(s.System.Update.State.Status, err)

				break
			}

			if newAppVersion != "" {
				appsUpdated[appName] = newAppVersion
			}
		}

		// Apply the system extensions.
		if len(appsUpdated) > 0 {
			slog.DebugContext(ctx, "Refreshing system extensions")

			err := systemd.RefreshExtensions(ctx)
			if err != nil {
				s.System.Update.State.Status = "Failed to refresh system extensions"
				showModalError(s.System.Update.State.Status, err)

				if isStartupCheck || isUserRequested {
					break
				}

				continue
			}
		}

		// Check for the latest OS update.
		newInstalledOSVersion, err := checkDownloadUpdate(ctx, s, t, p, "OS", "", isStartupCheck)
		if err != nil {
			s.System.Update.State.Status = "Failed to check for OS updates"
			showModalError(s.System.Update.State.Status, err)

			if isStartupCheck || isUserRequested {
				break
			}

			continue
		}

		// Notify the applications that they need to update/restart.
		for appName, appVersion := range appsUpdated {
			// Get the application.
			app, err := applications.Load(ctx, s, appName)
			if err != nil {
				s.System.Update.State.Status = "Failed to load application"
				showModalError(s.System.Update.State.Status, err)

				continue
			}

			// Start/reload the application.
			if !isStartupCheck {
				if app.IsRunning(ctx) {
					slog.InfoContext(ctx, "Reloading application", "name", appName, "version", appVersion)

					err := app.Update(ctx)
					if err != nil {
						s.System.Update.State.Status = "Failed to reload application"
						showModalError(s.System.Update.State.Status, err)

						continue
					}
				} else {
					err := applications.StartInitialize(ctx, s, appName)
					if err != nil {
						s.System.Update.State.Status = "Failed to start application"
						showModalError(s.System.Update.State.Status, err)

						continue
					}
				}
			}
		}

		updateModal := t.GetModal("update")

		if newInstalledOSVersion != "" {
			if updateModal == nil {
				updateModal = t.AddModal(s.OS.Name+" Update", "update")
			}

			s.System.Update.State.Status = s.OS.Name + " has been updated to version " + newInstalledOSVersion
			updateModal.Update(s.OS.Name + " has been updated to version " + newInstalledOSVersion + ".\nPlease reboot the system to finalize update.")
		} else {
			s.System.Update.State.Status = "Update check completed"

			if updateModal != nil {
				updateModal.Done()
			}
		}

		if isStartupCheck || isUserRequested {
			// If running a one-time update, we're done.
			break
		}
	}
}

func checkDownloadUpdate(ctx context.Context, s *state.State, t *tui.TUI, p providers.Provider, updateType string, appName string, isStartupCheck bool) (string, error) {
	s.UpdateMutex.Lock()
	defer s.UpdateMutex.Unlock()

	slog.DebugContext(ctx, "Checking for "+updateType+" updates")

	if s.System.Update.State.NeedsReboot {
		slog.DebugContext(ctx, "A reboot of the system is required to finalize a pending update")

		if updateType == "SecureBoot" {
			return "", nil
		}
	}

	// Get the appropriate update.
	var update providers.CommonUpdate

	var err error

	switch updateType {
	case "SecureBoot":
		update, err = p.GetSecureBootCertUpdate(ctx)
	case "OS":
		update, err = p.GetOSUpdate(ctx)
	case "application":
		update, err = p.GetApplicationUpdate(ctx, appName)
	default:
		return "", errors.New("unrecognized update type '" + updateType + "'")
	}

	if err != nil {
		if errors.Is(err, providers.ErrNoUpdateAvailable) {
			slog.DebugContext(ctx, updateType+" update provider doesn't currently have any update")

			return "", nil
		}

		return "", err
	}

	updateNeeded := false

	// Skip any update that isn't newer than what we are already running.
	switch update.(type) {
	case providers.SecureBootCertUpdate:
		updateNeeded = update.Version() != s.SecureBoot.Version

		if updateNeeded && s.SecureBoot.Version != "" && s.SecureBoot.Version != update.Version() && !update.IsNewerThan(s.SecureBoot.Version) {
			return "", errors.New("installed Secure Boot keys version (" + s.SecureBoot.Version + ") is newer than available update (" + update.Version() + "); skipping")
		}
	case providers.OSUpdate:
		// If we're running from the backup image don't attempt to re-update to a broken version.
		if !s.System.Update.State.NeedsReboot && s.OS.RunningFromBackup() && s.OS.NextRelease == update.Version() {
			slog.WarnContext(ctx, "Latest "+s.OS.Name+" image version "+s.OS.NextRelease+" has been identified as problematic, skipping update")

			return "", nil
		}

		updateNeeded = update.Version() != s.OS.RunningRelease && update.Version() != s.OS.NextRelease

		if updateNeeded && s.OS.RunningRelease != update.Version() && !update.IsNewerThan(s.OS.RunningRelease) {
			return "", errors.New("local " + s.OS.Name + " version (" + s.OS.RunningRelease + ") is newer than available update (" + update.Version() + "); skipping")
		}
	case providers.ApplicationUpdate:
		updateNeeded = update.Version() != s.Applications[appName].State.Version

		if updateNeeded && s.Applications[appName].State.Version != "" && !update.IsNewerThan(s.Applications[appName].State.Version) {
			return "", errors.New("local application " + appName + " version (" + s.Applications[appName].State.Version + ") is newer than available update (" + update.Version() + "); skipping")
		}
	default:
	}

	// Apply the update.
	if updateNeeded {
		return applyUpdate(ctx, s, t, update, updateType, appName, isStartupCheck)
	} else if isStartupCheck {
		_, isApplication := update.(providers.ApplicationUpdate)
		if isApplication {
			slog.DebugContext(ctx, "System is already running latest application version", "application", appName, "version", update.Version())
		} else {
			slog.DebugContext(ctx, "System is already running latest "+updateType+" version", "version", update.Version())
		}
	}

	return "", nil
}

func applyUpdate(ctx context.Context, s *state.State, t *tui.TUI, update providers.CommonUpdate, updateType string, appName string, isStartupCheck bool) (string, error) {
	updateModal := t.GetModal("update")

	if t.GetModal("update") == nil {
		updateModal = t.AddModal(s.OS.Name+" Update", "update")
	}

	// Download the update.
	_, isApplication := update.(providers.ApplicationUpdate)
	if isApplication {
		slog.InfoContext(ctx, "Downloading "+updateType+" update", "application", appName, "version", update.Version())
		updateModal.Update("Downloading " + updateType + " update " + appName + " update " + update.Version())
	} else {
		slog.InfoContext(ctx, "Downloading "+updateType+" update", "version", update.Version())
		updateModal.Update("Downloading " + updateType + " update " + update.Version())
	}

	targetPath := ""

	switch update.(type) {
	case providers.SecureBootCertUpdate:
		targetPath = "/tmp/"
	case providers.OSUpdate:
		targetPath = systemd.SystemUpdatesPath
	case providers.ApplicationUpdate:
		targetPath = systemd.SystemExtensionsPath
	default:
	}

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
		err = systemd.VerifyExtension(ctx, filepath.Join(systemd.SystemExtensionsPath, appName+".raw"))
		if err != nil {
			return "", err
		}

		// Record newly installed application and save state to disk.
		newAppInfo := s.Applications[appName]
		newAppInfo.State.Version = update.Version()

		s.Applications[appName] = newAppInfo
		_ = s.Save()
	default:
	}

	return update.Version(), nil
}
