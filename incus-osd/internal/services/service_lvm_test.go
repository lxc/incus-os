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

func TestLVMLockspaces(t *testing.T) {
	t.Parallel()

	output := `VG incusos-cur lock_type=sanlock IwehIqUnR6CNvef1cKbW7SymGEZDJyXw
LS sanlock lvm_incusos-cur
LK VG un ver 0
LK GL un ver 0
LK LV ex ErQPcxTKW2hYUtKnKHIQcTz1DdJyDIVi
LK LV sh qKZEb5Jcz2Zkc6CJ1Ckzng9EP7wyANgE

VG local lock_type=none abc
`

	require.Equal(t, map[string]map[string]string{
		"incusos-cur": {"ErQPcxTKW2hYUtKnKHIQcTz1DdJyDIVi": "ex", "qKZEb5Jcz2Zkc6CJ1Ckzng9EP7wyANgE": "sh"},
	}, lvmLockspaces(output))
}

func TestSanlockLockspaces(t *testing.T) {
	t.Parallel()

	output := `daemon 5e14fdac-64ed-440c-aaa8-0ef5decf31a9.incusos-lts
p -1 helper
p 1415 lvmlockd
s lvm_incusos-lts:23:/dev/mapper/incusos--lts-lvmlock:0
r lvm_incusos-lts:for18T-Oscn-zUuZ-3KNV-arEr-ZIDx-NewQQ7:/dev/mapper/incusos--lts-lvmlock:75497472:SH p 1415
`

	require.Equal(t, map[string]bool{"incusos-lts": true}, sanlockLockspaces(output))
}
