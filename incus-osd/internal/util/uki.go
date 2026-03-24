package util

import (
	"errors"
	"os"
	"regexp"
	"strings"
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
	versionRegex := regexp.MustCompile(`^.+_(\d{12}).+efi$`)

	versionGroup := versionRegex.FindStringSubmatch(ukiName)
	if len(versionGroup) != 2 {
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
