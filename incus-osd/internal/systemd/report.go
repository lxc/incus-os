package systemd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lxc/incus/v7/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
)

// GenerateReport returns a systemd-report report, optionally restricted to the matching metric families.
func GenerateReport(ctx context.Context, matches []string) (*api.DebugReport, error) {
	args := append([]string{"--", "generate"}, matches...)

	output, err := subprocess.RunCommandContext(ctx, "/usr/lib/systemd/systemd-report", args...)
	if err != nil {
		return nil, err
	}

	report := &api.DebugReport{
		MediaType: "application/vnd.io.systemd.report",
		Metrics:   []api.DebugReportMetric{},
	}

	// Nothing is written when no metrics source matches.
	if strings.TrimSpace(output) == "" {
		return report, nil
	}

	raw := struct {
		MediaType string                  `json:"mediaType"` //nolint:tagliatelle
		Timestamp string                  `json:"timestamp"`
		Metrics   []api.DebugReportMetric `json:"metrics"`
	}{}

	err = json.Unmarshal([]byte(output), &raw)
	if err != nil {
		return nil, err
	}

	report.MediaType = raw.MediaType
	report.Timestamp = raw.Timestamp
	report.Metrics = raw.Metrics

	return report, nil
}
