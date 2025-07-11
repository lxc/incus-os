package seed

// Install represents the install seed.
type Install struct {
	Version string `json:"version" yaml:"version"`

	ForceInstall bool           `json:"force_install" yaml:"force_install"` // If true, ignore any existing data on target install disk.
	ForceReboot  bool           `json:"force_reboot"  yaml:"force_reboot"`  // If true, reboot the system automatically upon completion rather than waiting for the install media to be removed.
	Target       *InstallTarget `json:"target"        yaml:"target"`        // Optional selector for the target install disk; if not set, expect a single drive to be present.
}

// InstallTarget defines options used to select the target install disk.
type InstallTarget struct {
	ID string `json:"id" yaml:"id"` // Name as listed in /dev/disk/by-id/, glob supported.
}
