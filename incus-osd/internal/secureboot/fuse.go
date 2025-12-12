package secureboot

import (
	"os"
)

// BlowTrustedFuse blows a virtual fuse that indicates we can no longer trust this
// system's state, such as when a swtpm-backed TPM is used. There is no way to undo
// this, short of completely reinstalling the system.
func BlowTrustedFuse() error {
	_, err := os.Create("/etc/incusos-trusted-fuse-blown")

	return err
}

// IsTrustedFuseBlown returns a boolean indicating if the system state can be trusted.
func IsTrustedFuseBlown() bool {
	_, err := os.Stat("/etc/incusos-trusted-fuse-blown")

	return err == nil
}
