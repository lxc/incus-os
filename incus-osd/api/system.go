package api

// SystemNextBootVersion defines a struct that is used to specify what version of IncusOS to boot the next time the system starts.
//
// swagger:model
type SystemNextBootVersion struct {
	// If not specified or "newest", the system will attempt to boot into the newest installed OS version.
	// Values of "current" or "oldest" will change the boot target to reboot into the same version or
	// the prior installed version.
	NextBootVersion string `json:"next_boot_version,omitempty" yaml:"next_boot_version,omitempty"`
}
