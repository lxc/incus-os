package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
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
				State:     ZpoolFinished,
				Examined:  4268032,
				ToExamine: 4276224,
			},
			expected: "100.00%",
		},
		{
			name: "Scanning returns current progress",
			stats: zpoolScanStats{
				State:     ZpoolScanning,
				Examined:  4268032,
				ToExamine: 4276224,
			},
			expected: "99.81%",
		},
		{
			name: "Scanning with progress overflow",
			stats: zpoolScanStats{
				State:     ZpoolScanning,
				Examined:  5268081,
				ToExamine: 4276224,
			},
			expected: "99.99%",
		},
		{
			name: "Scanning with no reported ToExamine",
			stats: zpoolScanStats{
				State:     ZpoolScanning,
				Examined:  5268081,
				ToExamine: 0,
			},
			expected: "0.00%",
		},
		{
			name: "Scanning with no reported Examined",
			stats: zpoolScanStats{
				State:     ZpoolScanning,
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
