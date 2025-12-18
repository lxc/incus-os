package seed

// Install represents the install seed.
type Install struct {
	Version string `json:"version" yaml:"version"`

	ForceInstall bool             `json:"force_install"      yaml:"force_install"`      // If true, ignore any existing data on target install disk.
	ForceReboot  bool             `json:"force_reboot"       yaml:"force_reboot"`       // If true, reboot the system automatically upon completion rather than waiting for the install media to be removed.
	Security     *InstallSecurity `json:"security,omitempty" yaml:"security,omitempty"` // Optional install options to allow IncusOS to run in a degraded security state.
	Target       *InstallTarget   `json:"target"             yaml:"target"`             // Optional selector for the target install disk; if not set, expect a single drive to be present.
}

// InstallSecurity defines a set of mutually exclusive options that allow IncusOS to run in a degraded security state.
// !!THESE OPTIONS WILL REDUCE THE SYSTEM'S SECURITY COMPARED TO USING PROPERLY CONFIGURED SECURE BOOT AND A TPM!!
type InstallSecurity struct {
	MissingTPM        bool `json:"missing_tpm"         yaml:"missing_tpm"`         // If true, and only if no physical TPM is present, allow fallback to swtpm-backed TPM implementation.
	MissingSecureBoot bool `json:"missing_secure_boot" yaml:"missing_secure_boot"` // If true, and only if Secure Boot is in a disabled state, allow fallback to booting without Secure Boot checks.
}

// InstallTarget defines options used to select the target install disk.
type InstallTarget struct {
	ID string `json:"id" yaml:"id"` // Name as listed in /dev/disk/by-id/, glob supported.
}
