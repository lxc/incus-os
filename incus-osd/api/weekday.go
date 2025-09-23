package api

import (
	"time"
)

// Weekday defines our own type. The time package's Weekday doesn't include any way to indicate
// an empty value, which we need.
type Weekday string

// Names of each day of the week.
const (
	NONE      Weekday = ""
	Sunday    Weekday = "Sunday"
	Monday    Weekday = "Monday"
	Tuesday   Weekday = "Tuesday"
	Wednesday Weekday = "Wednesday"
	Thursday  Weekday = "Thursday"
	Friday    Weekday = "Friday"
	Saturday  Weekday = "Saturday"
)

// ToWeekday converts our string representation into the time package's int-based Weekeday.
// It is assumed the value has already been checked to be a valid weekday, as no such
// error checking is performed here.
func (w Weekday) ToWeekday() time.Weekday {
	switch w { //nolint:exhaustive
	case Sunday:
		return time.Sunday
	case Monday:
		return time.Monday
	case Tuesday:
		return time.Tuesday
	case Wednesday:
		return time.Wednesday
	case Thursday:
		return time.Thursday
	case Friday:
		return time.Friday
	case Saturday:
		return time.Saturday
	default:
		return -1
	}
}
