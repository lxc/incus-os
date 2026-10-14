package api

// DebugReportMetric represents a single metric from a systemd-report report.
type DebugReportMetric struct {
	Name   string         `json:"name"             yaml:"name"`
	Object string         `json:"object,omitempty" yaml:"object,omitempty"`
	Fields map[string]any `json:"fields,omitempty" yaml:"fields,omitempty"`
	Value  any            `json:"value"            yaml:"value"`
}

// DebugReport represents a systemd-report report.
//
// swagger:model
type DebugReport struct {
	MediaType string              `json:"media_type" yaml:"media_type"`
	Timestamp string              `json:"timestamp"  yaml:"timestamp"`
	Metrics   []DebugReportMetric `json:"metrics"    yaml:"metrics"`
}
