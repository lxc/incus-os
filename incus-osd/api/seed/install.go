package seed

// Install represents the install seed.
type Install struct {
	Version string `json:"version" yaml:"version"`

	ForceInstall             bool             `json:"force_install"                        yaml:"force_install"`                        // If true, ignore any existing data on target install disk.
	ForceInstallConfirmation string           `json:"force_install_confirmation,omitempty" yaml:"force_install_confirmation,omitempty"` // An optional value used to confirm re-installation of IncusOS.
	ForceReboot              bool             `json:"force_reboot"                         yaml:"force_reboot"`                         // If true, reboot the system automatically upon completion rather than waiting for the install media to be removed.
	Security                 *InstallSecurity `json:"security,omitempty"                   yaml:"security,omitempty"`                   // Optional install options to allow IncusOS to run in a degraded security state.
	Target                   *InstallTarget   `json:"target"                               yaml:"target"`                               // Optional selector for the target install disk; if not set, expect a single drive to be present.
}

// InstallSecurity defines a set of mutually exclusive options that allow IncusOS to run in a degraded security state.
// !!THESE OPTIONS WILL REDUCE THE SYSTEM'S SECURITY COMPARED TO USING PROPERLY CONFIGURED SECURE BOOT AND A TPM!!
type InstallSecurity struct {
	MissingTPM        bool `json:"missing_tpm"         yaml:"missing_tpm"`         // If true, and only if no physical TPM is present, allow fallback to swtpm-backed TPM implementation.
	MissingSecureBoot bool `json:"missing_secure_boot" yaml:"missing_secure_boot"` // If true, and only if Secure Boot is in a disabled state, allow fallback to booting without Secure Boot checks.
}

// InstallTarget defines options used to select the target install disk.
type InstallTarget struct {
	// The following options are logically AND'ed together when selecting between more than one possible install targets.
	Bus     string `json:"bus,omitempty"      yaml:"bus,omitempty"`      // Bus type of the disk, for example "NVME", "SCSI", or "USB" (case insensitive).
	ID      string `json:"id,omitempty"       yaml:"id,omitempty"`       // Disk ID as listed in /dev/disk/by-id/, will be used in a case-sensitive sub-string match.
	MaxSize string `json:"max_size,omitempty" yaml:"max_size,omitempty"` // Maximum size of the install disk, such as 1TiB.
	MinSize string `json:"min_size,omitempty" yaml:"min_size,omitempty"` // Minimum size of the install disk, such as 100GiB.

	// If defined, sort potential targets by their capacity and pick the first one.
	SortOrder string `json:"sort_order,omitempty" yaml:"sort_order,omitempty"` // Either "smallest" or "largest" to sort matching targets by their capacity.
}
