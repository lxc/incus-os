package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// Kernel.
type cmdAdminOSDebugKernel struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSDebugKernel) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("kernel")
	cmd.Short = "Show kernel debug information"

	cmd.Long = cli.FormatSection("Description", "Show kernel debug information")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugKernel) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	if len(args) > 0 {
		remote, _ = parseRemote(args[0])
	}

	// Prepare the URL.
	u, err := url.Parse("/os/1.0/debug/kernel")
	if err != nil {
		return err
	}

	values := u.Query()
	if c.os.flagTarget != "" {
		values.Set("target", c.os.flagTarget)
	}

	u.RawQuery = values.Encode()

	// Get the kernel debug information.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", u.String(), nil, nil, "")
	if err != nil {
		return err
	}

	var data api.DebugKernel

	err = resp.MetadataAsStruct(&data)
	if err != nil {
		return err
	}

	_, _ = fmt.Printf("Kernel version: %s\n", data.Version)    //nolint:forbidigo
	_, _ = fmt.Printf("Architecture: %s\n", data.Architecture) //nolint:forbidigo

	if data.CPUBaseline != "" {
		_, _ = fmt.Printf("CPU baseline: %s\n", data.CPUBaseline) //nolint:forbidigo
	}

	_, _ = fmt.Println() //nolint:forbidigo

	rows := make([][]string, 0, len(data.Modules))

	for _, module := range data.Modules {
		inUse := "NO"
		if module.InUse {
			inUse = "YES"
		}

		rows = append(rows, []string{module.Name, strings.Join(module.Dependencies, ", "), inUse})
	}

	header := []string{"NAME", "DEPENDENCIES", "IN USE"}

	return cli.RenderTable(os.Stdout, "table", header, rows, nil)
}
