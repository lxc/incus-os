package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
)

func TestCalculateScrubProgress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		stats    zpoolScanStats
		expected string
	}{
		{
			name: "Finished returns 100.00% regardless of values",
			stats: zpoolScanStats{
				State:     "FINISHED",
				Examined:  4268032,
				ToExamine: 4276224,
			},
			expected: "100.00%",
		},
		{
			name: "Scanning returns current progress",
			stats: zpoolScanStats{
				State:     "SCANNING",
				Examined:  4268032,
				ToExamine: 4276224,
			},
			expected: "99.81%",
		},
		{
			name: "Scanning with progress overflow",
			stats: zpoolScanStats{
				State:     "SCANNING",
				Examined:  5268081,
				ToExamine: 4276224,
			},
			expected: "99.99%",
		},
		{
			name: "Scanning with no reported ToExamine",
			stats: zpoolScanStats{
				State:     "SCANNING",
				Examined:  5268081,
				ToExamine: 0,
			},
			expected: "0.00%",
		},
		{
			name: "Scanning with no reported Examined",
			stats: zpoolScanStats{
				State:     "SCANNING",
				Examined:  0,
				ToExamine: 4276224,
			},
			expected: "0.00%",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := calculateScrubProgress(tc.stats)
			require.Equal(t, tc.expected, got, tc.name)
		})
	}
}

func TestCalculateTrimStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		leaves []zpoolTrimStats
		want   *api.SystemStoragePoolTrimStatus
	}{
		{
			name:   "no leaves",
			leaves: []zpoolTrimStats{},
			want:   nil,
		},
		{
			name: "untrimmed and unsupported",
			leaves: []zpoolTrimStats{
				{TrimState: "UNTRIMMED"},
				{TrimState: "COMPLETE", TrimNotSup: 1},
			},
			want: nil,
		},
		{
			name: "all unsupported",
			leaves: []zpoolTrimStats{
				{TrimState: "UNTRIMMED", TrimNotSup: 1},
				{TrimState: "COMPLETE", TrimNotSup: 1},
			},
			want: &api.SystemStoragePoolTrimStatus{
				State: api.SystemStoragePoolTrimUnsupported,
			},
		},
		{
			name: "all complete",
			leaves: []zpoolTrimStats{
				{TrimState: "COMPLETE", Trimmed: 100, ToTrim: 100, TrimTime: 1000},
				{TrimState: "COMPLETE", Trimmed: 50, ToTrim: 50, TrimTime: 2000, TrimErrors: 1},
			},
			want: &api.SystemStoragePoolTrimStatus{
				State:          api.SystemStoragePoolTrimFinished,
				LastActionTime: time.Unix(2000, 0),
				Progress:       "100.00%",
				Errors:         1,
			},
		},
		{
			name: "partially in progress",
			leaves: []zpoolTrimStats{
				{TrimState: "COMPLETE", Trimmed: 100, ToTrim: 100, TrimTime: 3000},
				{TrimState: "ACTIVE", Trimmed: 20, ToTrim: 100, TrimTime: 3100},
				{TrimState: "UNTRIMMED"},
			},
			want: &api.SystemStoragePoolTrimStatus{
				State:          api.SystemStoragePoolTrimInProgress,
				LastActionTime: time.Unix(3100, 0),
				Progress:       "60.00%",
				Errors:         0,
			},
		},
		{
			name: "suspended",
			leaves: []zpoolTrimStats{
				{TrimState: "SUSPENDED", Trimmed: 0, ToTrim: 0, TrimTime: 4000},
			},
			want: &api.SystemStoragePoolTrimStatus{
				State:          api.SystemStoragePoolTrimSuspended,
				LastActionTime: time.Unix(4000, 0),
				Progress:       "0.00%",
				Errors:         0,
			},
		},
		{
			name: "unknown state",
			leaves: []zpoolTrimStats{
				{TrimState: "SOMETHING", Trimmed: 10, ToTrim: 10, TrimTime: 5000},
			},
			want: &api.SystemStoragePoolTrimStatus{
				State:          api.SystemStoragePoolTrimUnknown,
				LastActionTime: time.Unix(5000, 0),
				Progress:       "100.00%",
				Errors:         0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, calculateTrimStatus(tc.leaves))
		})
	}
}

func TestParseZpoolStatus(t *testing.T) {
	t.Parallel()

	byID := func(names ...string) []string {
		ret := make([]string, 0, len(names))
		for _, name := range names {
			ret = append(ret, "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_"+name)
		}

		return ret
	}

	cases := []struct {
		fixture     string
		pool        string
		poolType    string
		specialType string
		devices     map[string][]string
	}{
		{
			fixture:  "raid0-healthy",
			pool:     "p0",
			poolType: "zfs-raid0",
			devices:  map[string][]string{"devices": byID("d1", "d2", "d3")},
		},
		{
			fixture:  "raid0-replacing",
			pool:     "p0",
			poolType: "zfs-raid0",
			devices:  map[string][]string{"devices": byID("d1", "d2", "d3", "d4")},
		},
		{
			fixture:  "raidz1-replacing-faulted",
			pool:     "p1",
			poolType: "zfs-raidz1",
			devices:  map[string][]string{"devices": byID("d1", "d3", "d4", "d5"), "devices_degraded": byID("d2")},
		},
		{
			fixture:  "mirror-spare-active",
			pool:     "p2",
			poolType: "zfs-raid1",
			devices:  map[string][]string{"devices": byID("d2", "d3"), "devices_degraded": byID("d1")},
		},
		{
			fixture:     "mirror-log-special-replacing",
			pool:        "p3",
			poolType:    "zfs-raid1",
			specialType: "zfs-raid1",
			devices:     map[string][]string{"devices": byID("d1", "d2"), "log": byID("d3", "d4", "d7"), "special": byID("d5", "d6", "d8")},
		},
		{
			fixture:     "raid0-log-special-replacing",
			pool:        "p4",
			poolType:    "zfs-raid0",
			specialType: "zfs-raid0",
			devices:     map[string][]string{"devices": byID("d1"), "log": byID("d2", "d5"), "special": byID("d3", "d6"), "cache": byID("d4")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(filepath.Join("testdata", tc.fixture+".json"))
			require.NoError(t, err)

			zpoolJSON := zpoolStatusPartialParse{}
			require.NoError(t, json.Unmarshal(content, &zpoolJSON))

			layout, err := parseZpoolStatus(zpoolJSON, tc.pool)
			require.NoError(t, err)
			require.Equal(t, tc.poolType, layout.poolType)
			require.Equal(t, tc.specialType, layout.specialType)
			require.Equal(t, tc.devices, layout.devices)
		})
	}
}
