package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/lxc/incus/v6/shared/ask"
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
	usage := ""
	if c.os.args.SupportsRemote {
		usage = "[<remote>:]"
	}

	if c.entity != "" {
		usage += "<" + c.entityShort + ">"
	}

	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("edit", usage)
	cmd.Short = "Edit " + c.entity + " configuration"
	cmd.Long = cli.FormatSection("Description", "Edit "+c.entity+" configuration")
	cmd.Example = cli.FormatSection("", `incus admin os service edit `+usage+` < `+c.entityShort+`.yaml
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
		_, _, err := doQuery(c.os.args.DoHTTP, remote, "PUT", apiURL, os.Stdin, nil, "")

		return err
	}

	// Extract the current value
	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, nil, "")
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
			_, _, err = doQuery(c.os.args.DoHTTP, remote, "PUT", apiURL, makeJsonable(newdata), nil, "")
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
	usage := ""
	if c.os.args.SupportsRemote {
		usage = "[<remote>:]"
	}

	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("list", usage)
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
		data = append(data, []string{strings.TrimPrefix(v, "/os/1.0/"+c.endpoint+"/")})
	}

	sort.Sort(cli.SortColumnsNaturally(data))

	header := []string{
		"NAME",
	}

	return cli.RenderTable(os.Stdout, c.flagFormat, header, data, entries)
}

// Run.
type cmdGenericRun struct {
	action        string
	name          string
	description   string
	endpoint      string
	entity        string
	hasData       bool
	confirm       string
	defaultData   string
	hasFileInput  bool
	hasFileOutput bool

	flagData string

	os *cmdAdminOS
}

func (c *cmdGenericRun) command() *cobra.Command {
	cmd := &cobra.Command{}

	usage := ""
	if c.os.args.SupportsRemote {
		usage = "[<remote>:]"
	}

	if c.entity != "" {
		usage += "<" + c.entity + ">"
	}

	if c.hasFileOutput || c.hasFileInput {
		usage += " <file>"
	}

	if c.name == "" {
		c.name = c.action
	}

	cmd.Use = cli.Usage(c.name, usage)
	cmd.Short = c.description
	cmd.Long = cli.FormatSection("Description", c.description)

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	if c.hasData {
		cmd.Flags().StringVarP(&c.flagData, "data", "d", "", "Command data``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdGenericRun) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	minArgs := 0
	maxArgs := 1

	if c.entity != "" {
		minArgs++
	}

	if c.hasFileOutput || c.hasFileInput {
		minArgs++
		maxArgs++
	}

	exit, err := cli.CheckArgs(cmd, args, minArgs, maxArgs)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	resource := ""

	if len(args) > 0 {
		remote, resource = parseRemote(args[0])
	}

	if c.entity != "" && resource == "" {
		return errors.New("missing " + c.entity + " name")
	}

	// Ask for confirmation if needed.
	if c.confirm != "" {
		asker := ask.NewAsker(bufio.NewReader(os.Stdin))

		confirm, err := asker.AskBool(fmt.Sprintf("Are you sure you want to %s? (yes/no) [default=no]: ", c.confirm), "no")
		if err != nil {
			return err
		}

		if !confirm {
			return nil
		}
	}

	// Use cluster target if specified.
	apiURL := "/os/1.0/" + c.endpoint

	if c.entity != "" {
		apiURL += "/" + resource
	}

	apiURL += "/:" + c.action

	if c.os.flagTarget != "" {
		apiURL = apiURL + "?target=" + c.os.flagTarget
	}

	// Set default data.
	var inData any

	if c.hasData {
		if c.flagData == "" {
			c.flagData = c.defaultData
		}

		if c.flagData != "" {
			inData = c.flagData
		}
	}

	// Set source file.
	if c.hasFileInput {
		f, err := os.Open(args[len(args)-1])
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		inData = f
	}

	// Set target file.
	var outData io.Writer

	if c.hasFileOutput {
		f, err := os.Create(args[len(args)-1])
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		outData = f
	}

	// Run the command.
	_, _, err = doQuery(c.os.args.DoHTTP, remote, "POST", apiURL, inData, outData, "")
	if err != nil {
		return err
	}

	return nil
}

// Show.
type cmdGenericShow struct {
	os *cmdAdminOS

	endpoint    string
	entity      string
	entityShort string
}

func (c *cmdGenericShow) command() *cobra.Command {
	name := "Show details"

	usage := ""
	if c.os.args.SupportsRemote {
		usage = "[<remote>:]"
	}

	if c.entity != "" {
		name = "Show " + c.entity + " details"
		usage += "<" + c.entityShort + ">"
	}

	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("show", usage)
	cmd.Short = name
	cmd.Long = cli.FormatSection("Description", name)

	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdGenericShow) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	minArgs := 0
	maxArgs := 1

	if c.entity != "" {
		minArgs = 1
	}

	exit, err := cli.CheckArgs(cmd, args, minArgs, maxArgs)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	resource := ""

	if len(args) > 0 {
		remote, resource = parseRemote(args[0])
	}

	if c.entity != "" && resource == "" {
		return errors.New("missing " + c.entity + " name")
	}

	// Use cluster target if specified.
	apiURL := "/os/1.0"

	if c.endpoint != "" {
		apiURL += "/" + c.endpoint
	}

	if resource != "" {
		apiURL += "/" + resource
	}

	if c.os.flagTarget != "" {
		apiURL = apiURL + "?target=" + c.os.flagTarget
	}

	resp, _, err := doQuery(c.os.args.DoHTTP, remote, "GET", apiURL, nil, nil, "")
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
