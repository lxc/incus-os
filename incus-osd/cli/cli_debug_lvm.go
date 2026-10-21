package cli

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/lxc/incus/v7/shared/units"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// LVM.
type cmdAdminOSDebugLVM struct {
	os *cmdAdminOS

	flagFormat string
}

func (c *cmdAdminOSDebugLVM) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("lvm", c.os.remoteUsage())
	cmd.Short = "Show LVM debug information"

	cmd.Long = cli.FormatSection("Description", "Show LVM debug information")
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", c.os.args.DefaultListFormat, "Format (csv|json|table|yaml|compact|markdown), use suffix \",noheader\" to disable headers and \",header\" to enable it if missing, e.g. csv,header``")

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return cli.ValidateFlagFormatForListOutput(cmd.Flag("format").Value.String())
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugLVM) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	remote, exit, err := c.os.remoteArg(cmd, args)
	if exit {
		return err
	}

	// Prepare the URL.
	u, err := url.Parse("/os/1.0/debug/lvm")
	if err != nil {
		return err
	}

	values := u.Query()
	if c.os.flagTarget != "" {
		values.Set("target", c.os.flagTarget)
	}

	u.RawQuery = values.Encode()

	// Get the LVM debug information.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, nil, "")
	if err != nil {
		return err
	}

	var data api.DebugLVM

	err = resp.MetadataAsStruct(&data)
	if err != nil {
		return err
	}

	// Structured formats get the full reports.
	format, _, _ := strings.Cut(c.flagFormat, ",")
	if slices.Contains([]string{cli.TableFormatJSON, cli.TableFormatYAML}, format) {
		return cli.RenderTable(os.Stdout, c.flagFormat, nil, nil, data)
	}

	// Physical volumes table.
	pvRows := make([][]string, 0, len(data.PVs))

	for _, pv := range data.PVs {
		pvRows = append(pvRows, []string{
			lvmReportField(pv, "pv_name"),
			lvmReportField(pv, "vg_name"),
			lvmReportSize(pv, "pv_size"),
			lvmReportSize(pv, "pv_free"),
			lvmReportField(pv, "pv_attr"),
			lvmReportField(pv, "pv_uuid"),
		})
	}

	pvHeader := []string{"NAME", "VOLUME GROUP", "SIZE", "FREE", "ATTRIBUTES", "UUID"}

	_, _ = fmt.Println("Physical volumes:") //nolint:forbidigo

	err = cli.RenderTable(os.Stdout, c.flagFormat, pvHeader, pvRows, nil)
	if err != nil {
		return err
	}

	// Volume groups table.
	vgRows := make([][]string, 0, len(data.VGs))

	for _, vg := range data.VGs {
		vgRows = append(vgRows, []string{
			lvmReportField(vg, "vg_name"),
			lvmReportSize(vg, "vg_size"),
			lvmReportSize(vg, "vg_free"),
			lvmReportField(vg, "pv_count"),
			lvmReportField(vg, "lv_count"),
			lvmReportField(vg, "vg_lock_type"),
			lvmReportField(vg, "vg_systemid"),
			lvmReportField(vg, "vg_attr"),
			lvmReportField(vg, "vg_uuid"),
		})
	}

	vgHeader := []string{"NAME", "SIZE", "FREE", "PVS", "LVS", "LOCK TYPE", "SYSTEM ID", "ATTRIBUTES", "UUID"}

	_, _ = fmt.Println("\nVolume groups:") //nolint:forbidigo

	err = cli.RenderTable(os.Stdout, c.flagFormat, vgHeader, vgRows, nil)
	if err != nil {
		return err
	}

	// Logical volumes table.
	lvRows := make([][]string, 0, len(data.LVs))

	for _, lv := range data.LVs {
		lvRows = append(lvRows, []string{
			lvmReportField(lv, "lv_name"),
			lvmReportField(lv, "vg_name"),
			lvmReportSize(lv, "lv_size"),
			lvmReportField(lv, "lv_layout"),
			lvmReportField(lv, "pool_lv"),
			lvmReportField(lv, "origin"),
			lvmReportField(lv, "lv_attr"),
			lvmReportField(lv, "lv_uuid"),
		})
	}

	lvHeader := []string{"NAME", "VOLUME GROUP", "SIZE", "LAYOUT", "POOL", "ORIGIN", "ATTRIBUTES", "UUID"}

	_, _ = fmt.Println("\nLogical volumes:") //nolint:forbidigo

	return cli.RenderTable(os.Stdout, c.flagFormat, lvHeader, lvRows, nil)
}

// lvmReportField renders a report field as a string.
func lvmReportField(row map[string]any, key string) string {
	switch v := row[key].(type) {
	case nil:
		return ""

	case string:
		return v

	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)

	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, fmt.Sprint(item))
		}

		return strings.Join(items, ", ")

	default:
		return fmt.Sprint(v)
	}
}

// lvmReportSize renders a byte size field in a human readable form.
func lvmReportSize(row map[string]any, key string) string {
	value := lvmReportField(row, key)

	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return value
	}

	return units.GetByteSizeStringIEC(size, 2)
}
