package util

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"
)

// UKIVersions holds information about the UKI images present under /boot/EFI/Linux/.
type UKIVersions struct {
	CurrentVersion  string
	CurrentFilepath string
	OtherVersion    string
	OtherFilepath   string
}

// GetUKIVersions returns the version and file path for the currently running UKI. If a second
// UKI is present, its information will also be returned. This second UKI may be a prior version
// of IncusOS or an update that is pending a system reboot.
func GetUKIVersions() (UKIVersions, error) {
	ret := UKIVersions{}

	// Use the EFI variable LoaderEntrySelected to determine what UKI was booted.
	rawUKIName, err := ReadEFIVariable("LoaderEntrySelected")
	if err != nil {
		return ret, err
	}

	ukiName, err := UTF16ToString(rawUKIName)
	if err != nil {
		return ret, err
	}

	// Extract the IncusOS version that was booted. During OS upgrades, the EFI image is actually
	// renamed (see https://systemd.io/AUTOMATIC_BOOT_ASSESSMENT/#details for further details), so
	// pull out the 12-digit version which will be unique, then do a readdir to find the UKI image
	// we need to examine.
	// This logic can be simplified after December 2026 as the logic for using UKI profiles prevents
	// boot counting logic from actually running.
	versionRegex := regexp.MustCompile(`^.+_(\d{12}).+efi(@.+)?$`)

	versionGroup := versionRegex.FindStringSubmatch(ukiName)
	if len(versionGroup) != 3 {
		return ret, errors.New("unable to determine version from EFI variable LoaderEntrySelected ('" + ukiName + "')")
	}

	ukis, err := os.ReadDir("/boot/EFI/Linux/")
	if err != nil {
		return ret, err
	}

	for _, uki := range ukis {
		if strings.Contains(uki.Name(), versionGroup[1]) {
			ret.CurrentVersion = versionGroup[1]
			ret.CurrentFilepath = "/boot/EFI/Linux/" + uki.Name()
		} else {
			parts := strings.Split(uki.Name(), "_")

			if len(parts) != 2 {
				continue
			}

			ret.OtherVersion = strings.TrimSuffix(parts[1], ".efi")
			ret.OtherFilepath = "/boot/EFI/Linux/" + uki.Name()
		}
	}

	if ret.CurrentVersion == "" {
		return ret, errors.New("unable to find UKI image for version " + versionGroup[1])
	}

	return ret, nil
}

// GetCurrentUKIProfile inspects the kernel's command line to determine which UKI profile was
// booted. Unfortunately there doesn't seem to be any cleaner way to get this information.
func GetCurrentUKIProfile() (string, error) {
	cmdline, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return "", err
	}

	profileRegex := regexp.MustCompile(`incusos.profile=(\S+)`)
	profileGroup := profileRegex.FindStringSubmatch(string(cmdline))

	// If the regex doesn't match any profile, we booted the default "main" UKI profile.
	if len(profileGroup) != 2 {
		return "main", nil
	}

	return profileGroup[1], nil
}

// SetNextBootID determines the UKI image and profile that we should attempt to boot by
// default the next time the system starts. This helps ensure the system consistently uses
// the same UKI profile, which may influence kernel defaults or other system behavior.
func SetNextBootID(ctx context.Context) error {
	rebootID, err := getNextBootID(ctx)
	if err != nil {
		return err
	}

	_, err = subprocess.RunCommandContext(ctx, "bootctl", "set-oneshot", rebootID)
	if err != nil {
		return err
	}

	return nil
}

func getNextBootID(ctx context.Context) (string, error) {
	type bootctlEntry struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}

	entries := []bootctlEntry{}

	// Get a list of all available boot targets.
	output, err := subprocess.RunCommandContext(ctx, "bootctl", "list", "--json=short")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(output), &entries)
	if err != nil {
		return "", err
	}

	targets := []string{}
	targetRegex := regexp.MustCompile(`.*_\d{12}\.efi`)

	for _, entry := range entries {
		if entry.Type != "type2" {
			continue
		}

		if entry.ID != "" && targetRegex.FindString(entry.ID) != "" {
			targets = append(targets, entry.ID)
		}
	}

	// Ensure targets are sorted in reverse order, which will put the newest version first.
	slices.Sort(targets)
	slices.Reverse(targets)

	// Get the current UKI profile.
	profile, err := GetCurrentUKIProfile()
	if err != nil {
		return "", err
	}

	// Search for the first target that has the same profile.
	for _, target := range targets {
		if strings.HasSuffix(target, ".efi@"+profile) {
			return target, nil
		}
	}

	// Handle a legacy system that predates UKI profiles.
	// This check can be removed after December 2026.
	if profile == "main" {
		for _, target := range targets {
			if strings.HasSuffix(target, ".efi") {
				return target, nil
			}
		}
	}

	// We're unable to determine a boot target that matches our current profile.
	// This shouldn't happen, unless a profile is retired. In this case, return the
	// first UKI with the "main" profile, as that should hopefully be a sane choice.
	for _, target := range targets {
		if strings.HasSuffix(target, ".efi@main") {
			return target, nil
		}
	}

	// Shouldn't ever be able to reach this error.
	return "", errors.New("unable to identify a potential next boot UKI")
}
