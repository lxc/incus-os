package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// IncusOS license command.
type cmdAdminOSLicense struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSLicense) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("license")
	cmd.Short = "Show IncusOS license information"
	cmd.Long = cli.FormatSection("Description", "Show IncusOS license information")

	// List.
	listCmd := cmdAdminOSLicenseList{os: c.os}
	cmd.AddCommand(listCmd.command())

	// Show.
	showCmd := cmdAdminOSLicenseShow{os: c.os}
	cmd.AddCommand(showCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// IncusOS license list command.
type cmdAdminOSLicenseList struct {
	os *cmdAdminOS

	flagFormat string
}

func (c *cmdAdminOSLicenseList) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("list", c.os.remoteUsage())
	cmd.Aliases = []string{"ls"}
	cmd.Short = "List license entries"
	cmd.Long = cli.FormatSection("Description", "List license entries")
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

func (c *cmdAdminOSLicenseList) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	remote, exit, err := c.os.remoteArg(cmd, args)
	if exit {
		return err
	}

	// Use cluster target if specified.
	apiURL := "/os/1.0/licenses"
	if c.os.flagTarget != "" {
		apiURL += "?target=" + c.os.flagTarget
	}

	// Get the list.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, nil, "")
	if err != nil {
		return err
	}

	entries, err := resp.MetadataAsStringSlice()
	if err != nil {
		return err
	}

	data := [][]string{}

	for _, v := range entries {
		licenseType, name, _ := strings.Cut(strings.TrimPrefix(v, "/os/1.0/licenses/"), ":")
		data = append(data, []string{licenseType, name})
	}

	sort.Sort(cli.SortColumnsNaturally(data)) //nolint:revive

	header := []string{
		"TYPE",
		"NAME",
	}

	return cli.RenderTable(os.Stdout, c.flagFormat, header, data, entries)
}

// IncusOS license show command.
type cmdAdminOSLicenseShow struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSLicenseShow) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("show", c.os.remoteUsage()+"<type>", "<name>")
	cmd.Short = "Show the license text of an entry"
	cmd.Long = cli.FormatSection("Description", "Show the license text of an entry")

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSLicenseShow) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	licenseType := args[0]

	if c.os.args.SupportsRemote {
		remote, licenseType = parseRemote(args[0])
	}

	if licenseType == "" || args[1] == "" {
		return errors.New("missing license entry type or name")
	}

	name := licenseType + ":" + args[1]

	// Use cluster target if specified.
	apiURL := "/os/1.0/licenses/" + name
	if c.os.flagTarget != "" {
		apiURL += "?target=" + c.os.flagTarget
	}

	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, nil, "")
	if err != nil {
		return err
	}

	license := api.License{}

	err = resp.MetadataAsStruct(&license)
	if err != nil {
		return err
	}

	fmt.Print(license.Content) //nolint:forbidigo

	return nil
}
