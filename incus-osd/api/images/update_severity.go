package images

// UpdateSeverity represents the severity field in an update.
type UpdateSeverity string

const (
	// UpdateSeverityNone represents an unknown/unset severity.
	UpdateSeverityNone UpdateSeverity = "none"

	// UpdateSeverityLow represents the lowest severity.
	UpdateSeverityLow UpdateSeverity = "low"

	// UpdateSeverityMedium represents the medium severity.
	UpdateSeverityMedium UpdateSeverity = "medium"

	// UpdateSeverityHigh represents the high severity.
	UpdateSeverityHigh UpdateSeverity = "high"

	// UpdateSeverityCritical represents the critical severity.
	UpdateSeverityCritical UpdateSeverity = "critical"
)

// UpdateSeverities is a map of the supported update severities.
var UpdateSeverities = map[UpdateSeverity]struct{}{
	UpdateSeverityNone:     {},
	UpdateSeverityLow:      {},
	UpdateSeverityMedium:   {},
	UpdateSeverityHigh:     {},
	UpdateSeverityCritical: {},
}

func (u *UpdateSeverity) String() string {
	return string(*u)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (u *UpdateSeverity) MarshalText() ([]byte, error) {
	return []byte(*u), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (u *UpdateSeverity) UnmarshalText(text []byte) error {
	*u = UpdateSeverity(text)

	return nil
}
