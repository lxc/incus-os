package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
)

func TestLVMKilledVGs(t *testing.T) {
	t.Parallel()

	logs := []api.ServiceLVMLog{
		{LogMessage: "Skipping global lock: storage failed for sanlock leases"},
		{LogMessage: "VG incusos-lts lock skipped: storage failed for sanlock leases"},
		{LogMessage: "VG incusos-lts lock failed: storage failed for sanlock leases"},
		{LogMessage: "VG other lock skipped: storage errors for sanlock leases"},
		{LogMessage: "Reading VG incusos-lts without a lock."},
	}

	require.Equal(t, []string{"incusos-lts"}, lvmKilledVGs(logs))
}

func TestLVMStartedLockspaces(t *testing.T) {
	t.Parallel()

	output := `VG incusos-cur lock_type=sanlock IwehIqUnR6CNvef1cKbW7SymGEZDJyXw
LS sanlock lvm_incusos-cur
LK VG un ver 0
LK LV sh virtual-machines_v--rocky.block

VG local lock_type=none abc
`

	require.Equal(t, map[string]bool{"incusos-cur": true}, lvmStartedLockspaces(output))
}
