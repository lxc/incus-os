package applications

import (
	"context"
	"errors"
	"slices"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
	"github.com/lxc/incus-os/incus-osd/internal/state"
)

// Supported lists all supported applications.
var Supported = []string{"debug", "gpu-support", "incus", "incus-ceph", "incus-linstor", "migration-manager", "operations-center"}

// ErrNoPrimary is returned when the system doesn't yet have a primary application.
var ErrNoPrimary = errors.New("no primary application")

// Load retrieves and returns the application specific logic.
func Load(_ context.Context, s *state.State, name string) (Application, error) {
	var app Application

	switch name {
	case "debug":
		app = &debug{common: common{state: s, appState: &s.Applications.Debug.State}}
	case "gpu-support":
		app = &gpuSupport{common: common{state: s, appState: &s.Applications.GPUSupport.State}}
	case "incus":
		app = &incus{common: common{state: s, appState: &s.Applications.Incus.State.ApplicationState}}
	case "incus-ceph":
		app = &incusCeph{common: common{state: s, appState: &s.Applications.IncusCeph.State}}
	case "incus-linstor":
		app = &incusLinstor{common: common{state: s, appState: &s.Applications.IncusLinstor.State}}
	case "migration-manager":
		app = &migrationManager{common: common{state: s, appState: &s.Applications.MigrationManager.State}}
	case "operations-center":
		app = &operationsCenter{common: common{state: s, appState: &s.Applications.OperationsCenter.State}}
	default:
		return nil, errors.New("unknown application")
	}

	return app, nil
}

// GetPrimary returns the current primary application (optionally checking if initialized).
func GetPrimary(ctx context.Context, s *state.State, requireInitialized bool) (Application, error) {
	// Get the installed applications.
	apps, err := GetInstalled(ctx, s)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		// Skip uninitialized applications.
		if requireInitialized && !app.IsInitialized() {
			continue
		}

		// Check if a primary application.
		if app.IsPrimary() {
			return app, nil
		}
	}

	return nil, ErrNoPrimary
}

// GetInstalled returns a list of applications that are currently installed on the system.
func GetInstalled(ctx context.Context, s *state.State) ([]Application, error) {
	apps := []Application{}

	for _, appName := range Supported {
		app, err := Load(ctx, s, appName)
		if err != nil {
			return nil, err
		}

		if !app.IsInstalled() {
			continue
		}

		apps = append(apps, app)
	}

	return apps, nil
}

// GetInstallApplications returns a list of applications that should be installed on the system.
// If no applications are currently installed, attempt to get an application list from the seed.
// If no primary application is defined, the "incus" application will be automatically selected.
func GetInstallApplications(ctx context.Context, s *state.State) ([]string, error) {
	// Get the installed applications.
	apps, err := GetInstalled(ctx, s)
	if err != nil {
		return nil, err
	}

	// Build a list of applications to install.
	toInstall := []string{}

	if len(apps) == 0 {
		// Assume first start of the daemon.
		apps, err := seed.GetApplications(ctx)
		if err != nil && !seed.IsMissing(err) {
			return nil, errors.New("failed to get application list from seed: " + err.Error())
		}

		if apps != nil {
			// We have valid seed data.
			toInstall = make([]string, 0, len(apps.Applications))

			for _, app := range apps.Applications {
				toInstall = append(toInstall, app.Name)
			}
		}
	} else {
		// We have an existing application list.
		toInstall = make([]string, 0, len(apps))

		for _, app := range apps {
			toInstall = append(toInstall, app.Name())
		}
	}

	// Verify that at least one primary application is defined. If not, add incus to the list.
	foundPrimary := false

	for _, appName := range toInstall {
		app, err := Load(ctx, s, appName)
		if err == nil && app.IsPrimary() {
			foundPrimary = true

			break
		}
	}

	if !foundPrimary {
		toInstall = append(toInstall, "incus")
	}

	// Verify that each application has its dependencies, if any, included in the list of applications.
	for _, appName := range toInstall {
		app, err := Load(ctx, s, appName)
		if err != nil {
			return nil, errors.New("failed to check dependencies for application '" + appName + "': " + err.Error())
		}

		for _, dep := range app.GetDependencies() {
			if !slices.Contains(toInstall, dep) {
				toInstall = append(toInstall, dep)
			}
		}
	}

	// Sort and remove any duplicates.
	slices.Sort(toInstall)
	toInstall = slices.Compact(toInstall)

	return toInstall, nil
}
