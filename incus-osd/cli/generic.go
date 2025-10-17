package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	cli "github.com/lxc/incus/v6/shared/cmd"
	"github.com/lxc/incus/v6/shared/termios"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Edit.
type cmdGenericEdit struct {
	endpoint    string
	entity      string
	entityShort string

	os *cmdAdminOS
}

func (c *cmdGenericEdit) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("edit", "[<remote>:]<"+c.entityShort+">")
	cmd.Short = "Edit " + c.entity + " configuration"
	cmd.Long = cli.FormatSection("Description", "Edit "+c.entity+" configuration")
	cmd.Example = cli.FormatSection("", `incus admin os service edit [<remote>:]<`+c.entityShort+`> < `+c.entityShort+`.yaml
    Update an IncusOS `+c.entity+` using the content of `+c.entityShort+`.yaml.`)

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdGenericEdit) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote, resource := parseRemote(args[0])
	if resource == "" {
		return errors.New("missing " + c.entity + " name")
	}

	// Use cluster target if specified.
	apiURL := "/os/1.0/" + c.endpoint + "/" + resource
	if c.os.flagTarget != "" {
		apiURL = apiURL + "?target=" + c.os.flagTarget
	}

	// If stdin isn't a terminal, read text from it
	if !termios.IsTerminal(getStdinFd()) {
		_, _, err := doQuery(c.os.args.DoHTTP, remote, "PUT", apiURL, os.Stdin, "")

		return err
	}

	// Extract the current value
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, "")
	if err != nil {
		return err
	}

	var rawData any

	err = resp.MetadataAsStruct(&rawData)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(rawData)
	if err != nil {
		return err
	}

	// Spawn the editor
	content, err := cli.TextEditor("", []byte(string(data)))
	if err != nil {
		return err
	}

	for {
		// Parse the text received from the editor
		var newdata any

		err = yaml.Unmarshal(content, &newdata)
		if err == nil {
			_, _, err = doQuery(c.os.args.DoHTTP, remote, "PUT", apiURL, makeJsonable(newdata), "")
		}

		// Respawn the editor
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Config parsing error: %s\n", err)
			_, _ = fmt.Println("Press enter to open the editor again or ctrl+c to abort change") //nolint:forbidigo

			_, err := os.Stdin.Read(make([]byte, 1))
			if err != nil {
				return err
			}

			content, err = cli.TextEditor("", content)
			if err != nil {
				return err
			}

			continue
		}

		break
	}

	return nil
}

// List.
type cmdGenericList struct {
	os *cmdAdminOS

	endpoint string
	entity   string

	flagFormat string
}

func (c *cmdGenericList) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("list")
	cmd.Aliases = []string{"ls"}
	cmd.Short = "List " + c.entity
	cmd.Long = cli.FormatSection("Description", "List "+c.entity)
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", c.os.args.DefaultListFormat, "Format (csv|json|table|yaml|compact|markdown), use suffix \",noheader\" to disable headers and \",header\" to enable it if missing, e.g. csv,header``")

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return cli.ValidateFlagFormatForListOutput(cmd.Flag("format").Value.String())
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdGenericList) run(cmd *cobra.Command, args []string) error {
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

	// Use cluster target if specified.
	apiURL := "/os/1.0/" + c.endpoint
	if c.os.flagTarget != "" {
		apiURL += "?target=" + c.os.flagTarget
	}

	// Get the list.
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, "")
	if err != nil {
		return err
	}

	entries, err := resp.MetadataAsStringSlice()
	if err != nil {
		return err
	}

	data := [][]string{}
	for _, v := range entries {
		data = append(data, []string{strings.TrimPrefix(v, "/os/1.0/"+c.endpoint+"/")})
	}

	sort.Sort(cli.SortColumnsNaturally(data))

	header := []string{
		"NAME",
	}

	return cli.RenderTable(os.Stdout, c.flagFormat, header, data, entries)
}

// Show.
type cmdGenericShow struct {
	os *cmdAdminOS

	endpoint    string
	entity      string
	entityShort string
}

func (c *cmdGenericShow) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("show", "[<remote>:]<"+c.entityShort+">")
	cmd.Short = "Show " + c.entity + " details"
	cmd.Long = cli.FormatSection("Description", "Show "+c.entity+" details")

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdGenericShow) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote, resource := parseRemote(args[0])
	if resource == "" {
		return errors.New("missing " + c.entity + " name")
	}

	// Use cluster target if specified.
	apiURL := "/os/1.0/" + c.endpoint + "/" + resource
	if c.os.flagTarget != "" {
		apiURL = apiURL + "?target=" + c.os.flagTarget
	}

	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, "")
	if err != nil {
		return err
	}

	var rawData any

	err = resp.MetadataAsStruct(&rawData)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(rawData)
	if err != nil {
		return err
	}

	_, _ = fmt.Printf("%s", data) //nolint:forbidigo

	return nil
}
