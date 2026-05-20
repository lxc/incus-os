package cli

import (
	"os"
	"slices"
	"strconv"
	"strings"

	incusapi "github.com/lxc/incus/v7/shared/api"
	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// renderNetworkInfo decodes a system/network response and renders it as a table.
func renderNetworkInfo(resp *incusapi.Response) error {
	var network api.SystemNetwork

	err := resp.MetadataAsStruct(&network)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(network.State.Interfaces))
	for name := range network.State.Interfaces {
		names = append(names, name)
	}

	slices.Sort(names)

	rows := make([][]string, 0, len(names))
	for _, name := range names {
		iface := network.State.Interfaces[name]

		rows = append(rows, []string{
			name,
			iface.Type,
			iface.State,
			iface.Hwaddr,
			strconv.Itoa(iface.MTU),
			iface.Speed,
			strings.Join(iface.Addresses, "\n"),
			strings.Join(iface.Roles, "\n"),
		})
	}

	header := []string{"NAME", "TYPE", "STATE", "HWADDR", "MTU", "SPEED", "ADDRESSES", "ROLES"}

	return cli.RenderTable(os.Stdout, "table", header, rows, nil)
}

// systemInfoNetworkCommand returns an info command for the system/network endpoint.
func systemInfoNetworkCommand(c *cmdAdminOS, endpoint string, description string) *cobra.Command {
	return (&cmdGenericInfo{
		os:          c,
		endpoint:    endpoint,
		description: description,
		handler:     renderNetworkInfo,
	}).command()
}
