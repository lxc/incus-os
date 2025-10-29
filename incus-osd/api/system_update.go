package api

import (
	"time"
)

// SystemUpdate defines a struct to hold information about the system's update policy.
type SystemUpdate struct {
	Config SystemUpdateConfig `json:"config" yaml:"config"`

	State SystemUpdateState `incusos:"-" json:"state" yaml:"state"`
}

// SystemUpdateConfig defines a struct to hold configuration details for the update checks.
type SystemUpdateConfig struct {
	AutoReboot         bool                            `json:"auto_reboot"                   yaml:"auto_reboot"`
	Channel            string                          `json:"channel"                       yaml:"channel"`
	CheckFrequency     string                          `json:"check_frequency"               yaml:"check_frequency"`
	MaintenanceWindows []SystemUpdateMaintenanceWindow `json:"maintenance_windows,omitempty" yaml:"maintenance_windows,omitempty"`
}

// SystemUpdateState holds information about the current update state.
type SystemUpdateState struct {
	LastCheck   time.Time `json:"last_check"   yaml:"last_check"`
	Status      string    `json:"status"       yaml:"status"`
	NeedsReboot bool      `json:"needs_reboot" yaml:"needs_reboot"`
}

// SystemUpdateMaintenanceWindow defines a maintenance window for when it is acceptable to check for and apply updates.
// StartDayOfWeek and EndDayOfWeek are optional, and if non-zero can be used to limit the migration window to certain day(s).
// Times are assumed to be in UTC.
type SystemUpdateMaintenanceWindow struct {
	StartDayOfWeek Weekday `json:"start_day_of_week,omitempty" yaml:"start_day_of_week,omitempty"`
	StartHour      int     `json:"start_hour"                  yaml:"start_hour"`
	StartMinute    int     `json:"start_minute"                yaml:"start_minute"`
	EndDayOfWeek   Weekday `json:"end_day_of_week,omitempty"   yaml:"end_day_of_week,omitempty"`
	EndHour        int     `json:"end_hour"                    yaml:"end_hour"`
	EndMinute      int     `json:"end_minute"                  yaml:"end_minute"`
}

// IsCurrentlyActive returns true if the maintenance window is active.
func (w *SystemUpdateMaintenanceWindow) IsCurrentlyActive() bool {
	return w.IsActive(time.Now().UTC())
}

// IsActive returns true if the maintenance window will be active at the given point in time.
func (w *SystemUpdateMaintenanceWindow) IsActive(t time.Time) bool {
	// Compute maintenance windows as the number of minutes since 00:00 on Sunday.
	// We don't care about actual dates, just (potentially) days of the week and
	// a start/end time for each window.
	startOffset := 0
	endOffset := 0

	if w.StartDayOfWeek == NONE && w.EndDayOfWeek == NONE { //nolint:gocritic,nestif
		// If no start day and end day are provided, the maintenance window should trigger
		// each day, so compute the offset based on the current day of week we've been given.
		startOffset += int(t.Weekday()) * 24 * 60
		endOffset += int(t.Weekday()) * 24 * 60

		// Handle a maintenance window that extends beyond one day.
		if w.EndHour*60+w.EndMinute < w.StartHour*60+w.StartMinute {
			if t.Hour()*60+t.Minute() < w.StartHour*60+w.StartMinute {
				startOffset -= 24 * 60
			} else {
				endOffset += 24 * 60
			}
		}
	} else if w.StartDayOfWeek == w.EndDayOfWeek {
		// Just a single day.
		startOffset += (int(w.StartDayOfWeek.ToWeekday())) * 24 * 60
		endOffset += (int(w.StartDayOfWeek.ToWeekday())) * 24 * 60
	} else {
		if w.StartDayOfWeek.ToWeekday() < w.EndDayOfWeek.ToWeekday() {
			startOffset += (int(w.StartDayOfWeek.ToWeekday())) * 24 * 60
			endOffset += (int(w.EndDayOfWeek.ToWeekday())) * 24 * 60
		} else {
			startOffset += (int(w.StartDayOfWeek.ToWeekday())) * 24 * 60
			endOffset += (int(w.EndDayOfWeek.ToWeekday()) + 7) * 24 * 60
		}
	}

	// Add the specific hour and minute values to the start and end.
	startOffset += w.StartHour*60 + w.StartMinute
	endOffset += w.EndHour*60 + w.EndMinute

	// Compute our current offset for comparison with the start and end.
	currentPosition := int(t.Weekday())*24*60 + t.Hour()*60 + t.Minute()
	if endOffset >= 7*24*60 && int(t.Weekday()) <= int(w.EndDayOfWeek.ToWeekday()) {
		currentPosition += 7 * 24 * 60
	}

	return startOffset <= currentPosition && currentPosition <= endOffset
}
