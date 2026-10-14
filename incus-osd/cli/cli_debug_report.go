package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// Report.
type cmdAdminOSDebugReport struct {
	os *cmdAdminOS

	flagFormat string
	flagMatch  []string
}

func (c *cmdAdminOSDebugReport) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("report", c.os.remoteUsage())
	cmd.Short = "Show the system metrics report"

	cmd.Long = cli.FormatSection("Description", "Show the system metrics report")
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", c.os.args.DefaultListFormat, "Format (csv|json|table|yaml|compact|markdown), use suffix \",noheader\" to disable headers and \",header\" to enable it if missing, e.g. csv,header``")
	cmd.Flags().StringArrayVarP(&c.flagMatch, "match", "m", nil, "Metric family or prefix to include, may be repeated``")

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return cli.ValidateFlagFormatForListOutput(cmd.Flag("format").Value.String())
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugReport) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	remote, exit, err := c.os.remoteArg(cmd, args)
	if exit {
		return err
	}

	// Prepare the URL.
	u, err := url.Parse("/os/1.0/debug/report")
	if err != nil {
		return err
	}

	values := u.Query()
	if c.os.flagTarget != "" {
		values.Set("target", c.os.flagTarget)
	}

	for _, match := range c.flagMatch {
		values.Add("match", match)
	}

	u.RawQuery = values.Encode()

	// Get the report.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, nil, "")
	if err != nil {
		return err
	}

	var data api.DebugReport

	err = resp.MetadataAsStruct(&data)
	if err != nil {
		return err
	}

	rows := make([][]string, 0, len(data.Metrics))

	for _, metric := range data.Metrics {
		fields := make([]string, 0, len(metric.Fields))

		for _, key := range slices.Sorted(maps.Keys(metric.Fields)) {
			fields = append(fields, key+"="+formatReportValue(metric.Fields[key]))
		}

		rows = append(rows, []string{metric.Name, metric.Object, strings.Join(fields, ", "), formatReportValue(metric.Value)})
	}

	header := []string{"NAME", "OBJECT", "FIELDS", "VALUE"}

	return cli.RenderTable(os.Stdout, c.flagFormat, header, rows, data)
}

// formatReportValue renders a metric value, using JSON for structured values.
func formatReportValue(value any) string {
	switch v := value.(type) {
	case string:
		return v

	case map[string]any, []any:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}

		return string(data)

	default:
		return fmt.Sprint(v)
	}
}
