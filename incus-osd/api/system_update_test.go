package api_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lxc/incus-os/incus-osd/api"
)

type testInfo struct {
	MigrationWindow api.SystemUpdateMaintenanceWindow
	Time            time.Time
	Result          bool
}

func TestMigrationWindows(t *testing.T) {
	t.Parallel()

	// Simple maintenance window with no wrap.
	mw1 := api.SystemUpdateMaintenanceWindow{
		StartHour:   10,
		StartMinute: 0,
		EndHour:     11,
		EndMinute:   15,
	}

	// Simple maintenance window with wrap.
	mw2 := api.SystemUpdateMaintenanceWindow{
		StartHour:   23,
		StartMinute: 30,
		EndHour:     2,
		EndMinute:   0,
	}

	// Maintenance window on single day with no wrap.
	mw3 := api.SystemUpdateMaintenanceWindow{
		StartDayOfWeek: api.Thursday,
		StartHour:      12,
		StartMinute:    0,
		EndDayOfWeek:   api.Thursday,
		EndHour:        12,
		EndMinute:      15,
	}

	// Maintenance window over multiple days with no wrap.
	mw4 := api.SystemUpdateMaintenanceWindow{
		StartDayOfWeek: api.Wednesday,
		StartHour:      0,
		StartMinute:    0,
		EndDayOfWeek:   api.Friday,
		EndHour:        23,
		EndMinute:      59,
	}

	// Maintenance window over multiple days with wrap.
	mw5 := api.SystemUpdateMaintenanceWindow{
		StartDayOfWeek: api.Saturday,
		StartHour:      0,
		StartMinute:    0,
		EndDayOfWeek:   api.Sunday,
		EndHour:        23,
		EndMinute:      59,
	}

	tests := []testInfo{
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 9, 59, 0, 0, time.UTC),
			Result:          false,
		},
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 10, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 10, 1, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 10, 45, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 11, 14, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 11, 15, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw1,
			Time:            time.Date(2025, 8, 28, 11, 16, 0, 0, time.UTC),
			Result:          false,
		},

		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 28, 23, 29, 0, 0, time.UTC),
			Result:          false,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 28, 23, 30, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 28, 23, 31, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 29, 0, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 29, 1, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 29, 1, 59, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 29, 2, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw2,
			Time:            time.Date(2025, 8, 29, 2, 1, 0, 0, time.UTC),
			Result:          false,
		},

		{
			MigrationWindow: mw3,
			Time:            time.Date(2025, 8, 28, 12, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw3,
			Time:            time.Date(2025, 8, 28, 12, 5, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw3,
			Time:            time.Date(2025, 8, 28, 12, 15, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw3,
			Time:            time.Date(2025, 8, 27, 12, 5, 0, 0, time.UTC),
			Result:          false,
		},
		{
			MigrationWindow: mw3,
			Time:            time.Date(2025, 8, 29, 12, 5, 0, 0, time.UTC),
			Result:          false,
		},

		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 26, 23, 59, 0, 0, time.UTC),
			Result:          false,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 27, 0, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 27, 15, 15, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 28, 6, 30, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 28, 21, 45, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 29, 13, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 29, 23, 59, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw4,
			Time:            time.Date(2025, 8, 30, 0, 0, 0, 0, time.UTC),
			Result:          false,
		},

		{
			MigrationWindow: mw5,
			Time:            time.Date(2025, 8, 29, 23, 59, 0, 0, time.UTC),
			Result:          false,
		},
		{
			MigrationWindow: mw5,
			Time:            time.Date(2025, 8, 30, 0, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw5,
			Time:            time.Date(2025, 8, 31, 0, 0, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw5,
			Time:            time.Date(2025, 8, 31, 23, 59, 0, 0, time.UTC),
			Result:          true,
		},
		{
			MigrationWindow: mw5,
			Time:            time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC),
			Result:          false,
		},
	}

	for i, tst := range tests {
		result := tst.MigrationWindow.IsActive(tst.Time)

		require.Equal(t, result, tst.Result, "Test %d failed", i)
	}
}
