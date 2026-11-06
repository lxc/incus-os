package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"

	cli "github.com/lxc/incus/v7/shared/cmd"
	"github.com/spf13/cobra"

	"github.com/lxc/incus-os/incus-osd/api"
)

// IncusOS debug network command.
type cmdAdminOSDebugNetwork struct {
	os *cmdAdminOS
}

func (c *cmdAdminOSDebugNetwork) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage("network")
	cmd.Short = "Debug IncusOS networking"
	cmd.Long = cli.FormatSection("Description", "Debug IncusOS networking")

	// Dump traffic.
	dumpTrafficCmd := cmdAdminOSDebugNetworkDumpTraffic{os: c.os}
	cmd.AddCommand(dumpTrafficCmd.command())

	// Ping.
	pingCmd := cmdAdminOSDebugNetworkPing{os: c.os}
	cmd.AddCommand(pingCmd.command())

	// Reports.
	reports := []struct {
		name      string
		short     string
		hasFormat bool
	}{
		{"conntrack", "Get the connection tracking table", false},
		{"firewall", "Get the firewall ruleset", true},
		{"routes", "Get the routing rules and tables", true},
		{"sockets", "Get the system sockets", false},
	}

	for _, report := range reports {
		reportCmd := cmdAdminOSDebugNetworkReport{os: c.os, name: report.name, short: report.short, hasFormat: report.hasFormat}
		cmd.AddCommand(reportCmd.command())
	}

	// Speed test.
	speedTestCmd := cmdAdminOSDebugNetworkSpeedTest{os: c.os}
	cmd.AddCommand(speedTestCmd.command())

	// Trace.
	traceCmd := cmdAdminOSDebugNetworkTrace{os: c.os}
	cmd.AddCommand(traceCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706.
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, _ []string) { _ = cmd.Usage() }

	return cmd
}

// debugNetworkQuery queries a network debug endpoint and prints its result.
// Streamed output goes straight to outData, text results are printed as is and anything else as JSON.
func (c *cmdAdminOS) debugNetworkQuery(remote string, method string, name string, format string, data any, outData io.Writer) error {
	u, err := url.Parse("/os/1.0/debug/network/" + name)
	if err != nil {
		return err
	}

	values := u.Query()
	if c.flagTarget != "" {
		values.Set("target", c.flagTarget)
	}

	if format != "" {
		values.Set("format", format)
	}

	u.RawQuery = values.Encode()

	var inData any

	if data != nil {
		body, err := json.Marshal(data)
		if err != nil {
			return err
		}

		inData = string(body)
	}

	resp, _, err := doQuery(c.args.DoHTTP, remote, method, u.String(), inData, outData, "")
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	var text string

	err = resp.MetadataAsStruct(&text)
	if err == nil {
		_, _ = fmt.Print(text) //nolint:forbidigo

		return nil
	}

	var buf bytes.Buffer

	err = json.Indent(&buf, resp.Metadata, "", "  ")
	if err != nil {
		return err
	}

	_, _ = fmt.Println(buf.String()) //nolint:forbidigo

	return nil
}

// debugNetworkTarget splits the remote from the target of a command, keeping bare IPv6 addresses whole.
func (*cmdAdminOS) debugNetworkTarget(arg string) (string, string) {
	if net.ParseIP(arg) != nil {
		return "", arg
	}

	return parseRemote(arg)
}

// debugNetworkFormat checks the format requested for a command that supports both JSON and text output.
func (*cmdAdminOS) debugNetworkFormat(format string) error {
	if format != "json" && format != "text" {
		return errors.New("format must be either \"json\" or \"text\"")
	}

	return nil
}

// Dump traffic.
type cmdAdminOSDebugNetworkDumpTraffic struct {
	os *cmdAdminOS

	flagInterface string
	flagFilter    string
	flagFormat    string
	flagDuration  int
}

func (c *cmdAdminOSDebugNetworkDumpTraffic) command() *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use = cli.Usage("dump-traffic", c.os.remoteUsage()+"[<file|->]")
	cmd.Short = "Get a network traffic dump"

	cmd.Long = cli.FormatSection("Description", "Get a network traffic dump")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.Flags().StringVarP(&c.flagInterface, "interface", "i", "", "Interface to capture on``")
	cmd.Flags().StringVarP(&c.flagFilter, "filter", "f", "", "Capture filter expression``")
	cmd.Flags().StringVar(&c.flagFormat, "format", "text", "Format (text|pcap)")
	cmd.Flags().IntVarP(&c.flagDuration, "duration", "t", 10, "Capture duration in seconds, 0 for unlimited``")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugNetworkDumpTraffic) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote := ""
	path := ""

	if len(args) > 0 {
		remote, path = parseRemote(args[0])
	}

	// Set target file.
	outData := io.Writer(os.Stdout)

	if path != "" && path != "-" {
		_, err := os.Stat(path)
		if err == nil {
			return errors.New("output file '" + path + "' already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}

		defer func() { _ = f.Close() }()

		outData = f
	}

	data := api.DebugNetworkDumpTraffic{
		Interface: c.flagInterface,
		Filter:    c.flagFilter,
		Format:    c.flagFormat,
		Duration:  c.flagDuration,
	}

	return c.os.debugNetworkQuery(remote, "POST", ":dump-traffic", "", data, outData)
}

// Ping.
type cmdAdminOSDebugNetworkPing struct {
	os *cmdAdminOS

	flagCount         int
	flagFamily        string
	flagInterface     string
	flagInterval      float64
	flagNumeric       bool
	flagPMTUDiscovery string
	flagSize          int
}

func (c *cmdAdminOSDebugNetworkPing) command() *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use = cli.Usage("ping", c.os.remoteUsage()+"<target>")
	cmd.Short = "Ping a target from the system"

	cmd.Long = cli.FormatSection("Description", "Ping a target from the system")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.Flags().IntVarP(&c.flagCount, "count", "c", 10, "Number of pings to send, 0 for unlimited``")
	cmd.Flags().StringVar(&c.flagFamily, "family", "", "Address family (ipv4|ipv6)``")
	cmd.Flags().StringVarP(&c.flagInterface, "interface", "I", "", "Interface to send from``")
	cmd.Flags().Float64VarP(&c.flagInterval, "interval", "i", 0, "Seconds to wait between pings``")
	cmd.Flags().BoolVarP(&c.flagNumeric, "numeric", "n", false, "Don't resolve addresses to names")
	cmd.Flags().StringVarP(&c.flagPMTUDiscovery, "pmtu-discovery", "M", "", "Path MTU discovery strategy (do|dont|want|probe)``")
	cmd.Flags().IntVarP(&c.flagSize, "size", "s", 0, "Payload size in bytes``")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugNetworkPing) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote.
	remote, target := c.os.debugNetworkTarget(args[0])

	data := api.DebugNetworkPing{
		Target:        target,
		Family:        c.flagFamily,
		Interface:     c.flagInterface,
		Numeric:       c.flagNumeric,
		Count:         c.flagCount,
		Interval:      c.flagInterval,
		Size:          c.flagSize,
		PMTUDiscovery: c.flagPMTUDiscovery,
	}

	return c.os.debugNetworkQuery(remote, "POST", ":ping", "", data, nil)
}

// Reports.
type cmdAdminOSDebugNetworkReport struct {
	os *cmdAdminOS

	name      string
	short     string
	hasFormat bool

	flagFormat string
}

func (c *cmdAdminOSDebugNetworkReport) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = cli.Usage(c.name, c.os.remoteUsage())
	cmd.Short = c.short

	cmd.Long = cli.FormatSection("Description", c.short)
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	if c.hasFormat {
		cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "text", "Format (json|text)")
	}

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugNetworkReport) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	remote, exit, err := c.os.remoteArg(cmd, args)
	if exit {
		return err
	}

	if c.hasFormat {
		err = c.os.debugNetworkFormat(c.flagFormat)
		if err != nil {
			return err
		}
	}

	return c.os.debugNetworkQuery(remote, "GET", c.name, c.flagFormat, nil, nil)
}

// Speed test.
type cmdAdminOSDebugNetworkSpeedTest struct {
	os *cmdAdminOS

	flagBitrate   string
	flagDuration  int
	flagFamily    string
	flagFormat    string
	flagInterface string
	flagParallel  int
	flagPort      int
	flagProtocol  string
	flagReverse   bool
}

func (c *cmdAdminOSDebugNetworkSpeedTest) command() *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use = cli.Usage("speed-test", c.os.remoteUsage()+"<target>")
	cmd.Short = "Measure the bandwidth to an iperf3 server"

	cmd.Long = cli.FormatSection("Description", "Measure the bandwidth to an iperf3 server")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.Flags().StringVarP(&c.flagBitrate, "bitrate", "b", "", "Target bitrate in bits per second (K, M or G suffix allowed), 0 for unlimited``")
	cmd.Flags().IntVarP(&c.flagDuration, "duration", "t", 10, "Test duration in seconds, 0 for unlimited``")
	cmd.Flags().StringVar(&c.flagFamily, "family", "", "Address family (ipv4|ipv6)``")
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "text", "Format (json|text)")
	cmd.Flags().StringVarP(&c.flagInterface, "interface", "I", "", "Interface to send from``")
	cmd.Flags().IntVarP(&c.flagParallel, "parallel", "P", 0, "Number of parallel connections``")
	cmd.Flags().IntVarP(&c.flagPort, "port", "p", 0, "Server port``")
	cmd.Flags().StringVar(&c.flagProtocol, "protocol", "", "Protocol (tcp|udp)``")
	cmd.Flags().BoolVarP(&c.flagReverse, "reverse", "R", false, "Measure the download rather than upload bandwidth")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugNetworkSpeedTest) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	err = c.os.debugNetworkFormat(c.flagFormat)
	if err != nil {
		return err
	}

	// Parse remote.
	remote, target := c.os.debugNetworkTarget(args[0])

	data := api.DebugNetworkSpeedTest{
		Target:    target,
		Port:      c.flagPort,
		Family:    c.flagFamily,
		Interface: c.flagInterface,
		Protocol:  c.flagProtocol,
		Bitrate:   c.flagBitrate,
		Parallel:  c.flagParallel,
		Duration:  c.flagDuration,
		Reverse:   c.flagReverse,
		Format:    c.flagFormat,
	}

	return c.os.debugNetworkQuery(remote, "POST", ":speed-test", "", data, nil)
}

// Trace.
type cmdAdminOSDebugNetworkTrace struct {
	os *cmdAdminOS

	flagCount     int
	flagFamily    string
	flagFormat    string
	flagInterface string
	flagNumeric   bool
}

func (c *cmdAdminOSDebugNetworkTrace) command() *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use = cli.Usage("trace", c.os.remoteUsage()+"<target>")
	cmd.Short = "Trace the network path to a target"

	cmd.Long = cli.FormatSection("Description", "Trace the network path to a target")
	if c.os.args.SupportsTarget {
		cmd.Flags().StringVar(&c.os.flagTarget, "target", "", "Cluster member name``")
	}

	cmd.Flags().IntVarP(&c.flagCount, "count", "c", 10, "Number of probes to send to each hop, 0 for unlimited``")
	cmd.Flags().StringVar(&c.flagFamily, "family", "", "Address family (ipv4|ipv6)``")
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "text", "Format (json|text)")
	cmd.Flags().StringVarP(&c.flagInterface, "interface", "I", "", "Interface to send from``")
	cmd.Flags().BoolVarP(&c.flagNumeric, "numeric", "n", false, "Don't resolve addresses to names")

	cmd.RunE = c.run

	return cmd
}

func (c *cmdAdminOSDebugNetworkTrace) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := cli.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	err = c.os.debugNetworkFormat(c.flagFormat)
	if err != nil {
		return err
	}

	// Parse remote.
	remote, target := c.os.debugNetworkTarget(args[0])

	data := api.DebugNetworkTrace{
		Target:    target,
		Family:    c.flagFamily,
		Interface: c.flagInterface,
		Numeric:   c.flagNumeric,
		Count:     c.flagCount,
		Format:    c.flagFormat,
	}

	return c.os.debugNetworkQuery(remote, "POST", ":trace", "", data, nil)
}
