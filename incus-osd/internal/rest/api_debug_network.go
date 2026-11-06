package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lxc/incus/v7/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/rest/response"
)

// debugNetworkDefaultLimit is the count or duration used by the network debug commands when none is requested.
const debugNetworkDefaultLimit = 10

var debugNetworkBitrateRegexp = regexp.MustCompile(`^[0-9]+[kKmMgG]?$`)

// debugFlushWriter flushes the response after every write so that streamed output shows up as it's produced.
type debugFlushWriter struct {
	w          http.ResponseWriter
	controller *http.ResponseController
}

func (f *debugFlushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	_ = f.controller.Flush()

	return n, err
}

// requireDebugTool checks that the named tool is installed, rendering an error if it isn't.
func requireDebugTool(w http.ResponseWriter, name string) bool {
	_, err := exec.LookPath(name)
	if err != nil {
		_ = response.PreconditionFailed(errors.New("\"" + name + "\" isn't available, install the \"debug\" application")).Render(w)

		return false
	}

	return true
}

// debugNetworkArgs decodes the arguments of a network debug command on top of the provided defaults.
func debugNetworkArgs(w http.ResponseWriter, r *http.Request, args any) bool {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		_ = response.NotImplemented(nil).Render(w)

		return false
	}

	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024)).Decode(args)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return false
	}

	return true
}

// debugNetworkFamilyArg returns the command line option selecting the requested address family.
func debugNetworkFamilyArg(family string) ([]string, error) {
	switch family {
	case "":
		return nil, nil
	case "ipv4":
		return []string{"-4"}, nil
	case "ipv6":
		return []string{"-6"}, nil
	default:
		return nil, errors.New("family must be either \"ipv4\" or \"ipv6\"")
	}
}

// debugNetworkTarget makes sure that a target can't be mistaken for a command line option.
func debugNetworkTarget(target string) error {
	if target == "" {
		return errors.New("missing target")
	}

	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, " \t\n") {
		return errors.New("invalid target")
	}

	return nil
}

// debugNetworkInterface checks that a requested interface exists.
func debugNetworkInterface(name string) error {
	if name == "" {
		return nil
	}

	_, err := net.InterfaceByName(name)
	if err != nil {
		return fmt.Errorf("invalid interface %q: %w", name, err)
	}

	return nil
}

// debugNetworkStream runs a command, streaming its output to the client until it exits or the request is canceled.
func debugNetworkStream(ctx context.Context, w http.ResponseWriter, stderr bool, name string, args ...string) error {
	w.Header().Set("Content-Type", "application/octet-stream")

	out := &debugFlushWriter{w: w, controller: http.NewResponseController(w)}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	cmd.Stdout = out

	if stderr {
		cmd.Stderr = out
	}

	return cmd.Run()
}

// swagger:operation GET /1.0/debug/network debug debug_network_get
//
//	Get network debug endpoints
//
//	Returns a list of network debug endpoints (URLs).
//
//	These endpoints have no guarantee of API stability, and should not be used in normal day-to-day operations.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of network debug endpoints
//	          items:
//	            type: string
//	          example: ["/1.0/debug/network/sockets"]
func (*Server) apiDebugNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	endpoint, _ := url.JoinPath(getAPIRoot(r), "debug", "network")

	urls := []string{}

	for _, debug := range []string{":dump-traffic", ":ping", ":speed-test", ":trace", "conntrack", "firewall", "routes", "sockets"} {
		debugURL, _ := url.JoinPath(endpoint, debug)
		urls = append(urls, debugURL)
	}

	_ = response.SyncResponse(true, urls).Render(w)
}

// swagger:operation GET /1.0/debug/network/conntrack debug debug_network_get_conntrack
//
//	Get connection tracking table
//
//	Return the connection tracking table ("conntrack" output).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Text connection tracking table
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: string
//	          description: Text connection tracking table
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugNetworkConntrack(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	if !requireDebugTool(w, "conntrack") {
		return
	}

	output, err := subprocess.RunCommandContext(r.Context(), "conntrack", "-L")
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, output).Render(w)
}

// swagger:operation GET /1.0/debug/network/firewall debug debug_network_get_firewall
//
//	Get firewall ruleset
//
//	Return the full nftables ruleset ("nft list ruleset" output), as JSON unless the text format is requested.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: format
//	    description: Output format, either "json" (default) or "text"
//	    type: string
//	responses:
//	  "200":
//	    description: Firewall ruleset
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: object
//	          description: Firewall ruleset
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugNetworkFirewall(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	switch r.URL.Query().Get("format") {
	case "", "json":
		output, err := subprocess.RunCommandContext(r.Context(), "nft", "-j", "list", "ruleset")
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.SyncResponse(true, json.RawMessage(output)).Render(w)
	case "text":
		output, err := subprocess.RunCommandContext(r.Context(), "nft", "list", "ruleset")
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.SyncResponse(true, output).Render(w)
	default:
		_ = response.BadRequest(errors.New("format must be either \"json\" or \"text\"")).Render(w)
	}
}

// swagger:operation GET /1.0/debug/network/routes debug debug_network_get_routes
//
//	Get routing tables
//
//	Return the IPv4 and IPv6 policy rules and routing tables ("ip rule" and "ip route" output), as JSON unless the text format is requested.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: format
//	    description: Output format, either "json" (default) or "text"
//	    type: string
//	responses:
//	  "200":
//	    description: Routing rules and tables
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/DebugNetworkRoutes"
//	          description: Routing rules and tables
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugNetworkRoutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	format := r.URL.Query().Get("format")
	if !slices.Contains([]string{"", "json", "text"}, format) {
		_ = response.BadRequest(errors.New("format must be either \"json\" or \"text\"")).Render(w)

		return
	}

	routes := api.DebugNetworkRoutes{}

	sections := []struct {
		title string
		args  []string
		dest  *[]map[string]any
	}{
		{"IPv4 rules", []string{"-4", "rule", "show"}, &routes.IPv4Rules},
		{"IPv4 routes", []string{"-4", "route", "show", "table", "all"}, &routes.IPv4Routes},
		{"IPv6 rules", []string{"-6", "rule", "show"}, &routes.IPv6Rules},
		{"IPv6 routes", []string{"-6", "route", "show", "table", "all"}, &routes.IPv6Routes},
	}

	var text strings.Builder

	for _, section := range sections {
		args := section.args
		if format != "text" {
			args = append([]string{"-j"}, args...)
		}

		output, err := subprocess.RunCommandContext(r.Context(), "ip", args...)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		if format == "text" {
			fmt.Fprintf(&text, "== %s ==\n%s\n", section.title, output)

			continue
		}

		err = json.Unmarshal([]byte(output), section.dest)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}
	}

	if format == "text" {
		_ = response.SyncResponse(true, text.String()).Render(w)

		return
	}

	_ = response.SyncResponse(true, routes).Render(w)
}

// swagger:operation GET /1.0/debug/network/sockets debug debug_network_get_sockets
//
//	Get socket list
//
//	Return the list of all system sockets ("ss" output).
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Text list of system sockets
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          description: Response type
//	          example: sync
//	          type: string
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: string
//	          description: Text list of system sockets
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugNetworkSockets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		_ = response.NotImplemented(nil).Render(w)

		return
	}

	output, err := subprocess.RunCommandContext(r.Context(), "ss", "-apn")
	if err != nil {
		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, output).Render(w)
}

// swagger:operation POST /1.0/debug/network/:ping debug debug_network_post_ping
//
//	Ping a target
//
//	Send ICMP echo requests to the provided target, streaming the text output until done or until the request is canceled.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/octet-stream
//	parameters:
//	  - in: body
//	    name: arguments
//	    description: Ping arguments
//	    required: true
//	    schema:
//	      $ref: "#/definitions/DebugNetworkPing"
//	responses:
//	  "200":
//	    description: Raw ping output
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
func (*Server) apiDebugNetworkPing(w http.ResponseWriter, r *http.Request) {
	args := api.DebugNetworkPing{Count: debugNetworkDefaultLimit}

	if !debugNetworkArgs(w, r, &args) || !requireDebugTool(w, "ping") {
		return
	}

	cmdArgs, err := debugNetworkFamilyArg(args.Family)
	if err == nil {
		err = debugNetworkTarget(args.Target)
	}

	if err == nil {
		err = debugNetworkInterface(args.Interface)
	}

	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if args.Count < 0 || args.Interval < 0 || args.Size < 0 || args.Size > 65507 {
		_ = response.BadRequest(errors.New("invalid count, interval or size")).Render(w)

		return
	}

	if !slices.Contains([]string{"", "do", "dont", "want", "probe"}, args.PMTUDiscovery) {
		_ = response.BadRequest(errors.New("pmtu_discovery must be one of \"do\", \"dont\", \"want\" or \"probe\"")).Render(w)

		return
	}

	if args.Interface != "" {
		cmdArgs = append(cmdArgs, "-I", args.Interface)
	}

	if args.Numeric {
		cmdArgs = append(cmdArgs, "-n")
	}

	if args.Count > 0 {
		cmdArgs = append(cmdArgs, "-c", strconv.Itoa(args.Count))
	}

	if args.Interval > 0 {
		cmdArgs = append(cmdArgs, "-i", strconv.FormatFloat(args.Interval, 'f', -1, 64))
	}

	if args.Size > 0 {
		cmdArgs = append(cmdArgs, "-s", strconv.Itoa(args.Size))
	}

	if args.PMTUDiscovery != "" {
		cmdArgs = append(cmdArgs, "-M", args.PMTUDiscovery)
	}

	_ = debugNetworkStream(r.Context(), w, true, "ping", append(cmdArgs, "--", args.Target)...)
}

// swagger:operation POST /1.0/debug/network/:speed-test debug debug_network_post_speed_test
//
//	Run a speed test
//
//	Measure the bandwidth to an iperf3 server. The JSON format returns the report once the test is done. The text format streams the output until done or until the request is canceled, and is the only one available for an unlimited duration.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/octet-stream
//	parameters:
//	  - in: body
//	    name: arguments
//	    description: Speed test arguments
//	    required: true
//	    schema:
//	      $ref: "#/definitions/DebugNetworkSpeedTest"
//	responses:
//	  "200":
//	    description: Speed test report, or raw output in text format
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugNetworkSpeedTest(w http.ResponseWriter, r *http.Request) {
	args := api.DebugNetworkSpeedTest{Duration: debugNetworkDefaultLimit}

	if !debugNetworkArgs(w, r, &args) || !requireDebugTool(w, "iperf3") {
		return
	}

	cmdArgs, err := debugNetworkFamilyArg(args.Family)
	if err == nil {
		err = debugNetworkTarget(args.Target)
	}

	if err == nil {
		err = debugNetworkInterface(args.Interface)
	}

	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if args.Duration < 0 || args.Port < 0 || args.Port > 65535 || args.Parallel < 0 || args.Parallel > 128 {
		_ = response.BadRequest(errors.New("invalid duration, port or parallel connection count")).Render(w)

		return
	}

	if !slices.Contains([]string{"", "tcp", "udp"}, args.Protocol) {
		_ = response.BadRequest(errors.New("protocol must be either \"tcp\" or \"udp\"")).Render(w)

		return
	}

	if args.Bitrate != "" && !debugNetworkBitrateRegexp.MatchString(args.Bitrate) {
		_ = response.BadRequest(errors.New("bitrate must be a number with an optional K, M or G suffix")).Render(w)

		return
	}

	if !slices.Contains([]string{"", "json", "text"}, args.Format) {
		_ = response.BadRequest(errors.New("format must be either \"json\" or \"text\"")).Render(w)

		return
	}

	if args.Format != "text" && args.Duration == 0 {
		_ = response.BadRequest(errors.New("an unlimited speed test requires the text format")).Render(w)

		return
	}

	cmdArgs = append(cmdArgs, "-c", args.Target, "-t", strconv.Itoa(args.Duration))

	if args.Interface != "" {
		cmdArgs = append(cmdArgs, "--bind-dev", args.Interface)
	}

	if args.Port > 0 {
		cmdArgs = append(cmdArgs, "-p", strconv.Itoa(args.Port))
	}

	if args.Protocol == "udp" {
		cmdArgs = append(cmdArgs, "-u")
	}

	if args.Bitrate != "" {
		cmdArgs = append(cmdArgs, "-b", args.Bitrate)
	}

	if args.Parallel > 0 {
		cmdArgs = append(cmdArgs, "-P", strconv.Itoa(args.Parallel))
	}

	if args.Reverse {
		cmdArgs = append(cmdArgs, "-R")
	}

	if args.Format == "text" {
		_ = debugNetworkStream(r.Context(), w, true, "iperf3", append(cmdArgs, "--forceflush")...)

		return
	}

	output, err := subprocess.RunCommandContext(r.Context(), "iperf3", append(cmdArgs, "-J")...)
	if err != nil {
		// The reason for a failure is part of the JSON report.
		report := struct {
			Error string `json:"error"`
		}{}

		if json.Unmarshal([]byte(output), &report) == nil && report.Error != "" {
			err = errors.New(report.Error)
		}

		_ = response.InternalError(err).Render(w)

		return
	}

	_ = response.SyncResponse(true, json.RawMessage(output)).Render(w)
}

// swagger:operation POST /1.0/debug/network/:trace debug debug_network_post_trace
//
//	Trace the path to a target
//
//	Trace the network path to the provided target. The JSON format returns the report once the trace is done. The text format streams a report for every round of probes until done or until the request is canceled, and is the only one available for an unlimited count.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/octet-stream
//	parameters:
//	  - in: body
//	    name: arguments
//	    description: Trace arguments
//	    required: true
//	    schema:
//	      $ref: "#/definitions/DebugNetworkTrace"
//	responses:
//	  "200":
//	    description: Trace report, or raw output in text format
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func (*Server) apiDebugNetworkTrace(w http.ResponseWriter, r *http.Request) {
	args := api.DebugNetworkTrace{Count: debugNetworkDefaultLimit}

	if !debugNetworkArgs(w, r, &args) || !requireDebugTool(w, "mtr") {
		return
	}

	cmdArgs, err := debugNetworkFamilyArg(args.Family)
	if err == nil {
		err = debugNetworkTarget(args.Target)
	}

	if err == nil {
		err = debugNetworkInterface(args.Interface)
	}

	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	if args.Count < 0 {
		_ = response.BadRequest(errors.New("invalid count")).Render(w)

		return
	}

	if !slices.Contains([]string{"", "json", "text"}, args.Format) {
		_ = response.BadRequest(errors.New("format must be either \"json\" or \"text\"")).Render(w)

		return
	}

	if args.Format != "text" && args.Count == 0 {
		_ = response.BadRequest(errors.New("an unlimited trace requires the text format")).Render(w)

		return
	}

	if args.Interface != "" {
		cmdArgs = append(cmdArgs, "-I", args.Interface)
	}

	if args.Numeric {
		cmdArgs = append(cmdArgs, "-n")
	}

	if args.Format != "text" {
		output, err := subprocess.RunCommandContext(r.Context(), "mtr", append(cmdArgs, "--json", "-c", strconv.Itoa(args.Count), "--", args.Target)...)
		if err != nil {
			_ = response.InternalError(err).Render(w)

			return
		}

		_ = response.SyncResponse(true, json.RawMessage(output)).Render(w)

		return
	}

	cmdArgs = append(cmdArgs, "--report", "--report-wide")

	// A report only gets printed at the end, so an unlimited trace is a series of reports.
	count := args.Count
	if count == 0 {
		count = debugNetworkDefaultLimit
	}

	cmdArgs = append(cmdArgs, "-c", strconv.Itoa(count), "--", args.Target)

	for {
		err := debugNetworkStream(r.Context(), w, true, "mtr", cmdArgs...)
		if err != nil || args.Count > 0 || r.Context().Err() != nil {
			return
		}
	}
}

// swagger:operation POST /1.0/debug/network/:dump-traffic debug debug_network_post_dump_traffic
//
//	Get a traffic dump
//
//	Capture network traffic, streaming it as text or pcap data until done or until the request is canceled.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	  - application/octet-stream
//	parameters:
//	  - in: body
//	    name: arguments
//	    description: Traffic dump arguments
//	    required: true
//	    schema:
//	      $ref: "#/definitions/DebugNetworkDumpTraffic"
//	responses:
//	  "200":
//	    description: Raw traffic dump
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
func (*Server) apiDebugNetworkDumpTraffic(w http.ResponseWriter, r *http.Request) {
	args := api.DebugNetworkDumpTraffic{Duration: debugNetworkDefaultLimit}

	if !debugNetworkArgs(w, r, &args) || !requireDebugTool(w, "tcpdump") {
		return
	}

	if args.Interface == "" {
		args.Interface = "any"
	}

	if strings.HasPrefix(args.Interface, "-") || args.Duration < 0 {
		_ = response.BadRequest(errors.New("invalid interface or duration")).Render(w)

		return
	}

	cmdArgs := []string{"-i", args.Interface, "-n"}

	switch args.Format {
	case "", "text":
		cmdArgs = append(cmdArgs, "-l")
	case "pcap":
		cmdArgs = append(cmdArgs, "-U", "-w", "-")
	default:
		_ = response.BadRequest(errors.New("format must be either \"text\" or \"pcap\"")).Render(w)

		return
	}

	if args.Filter != "" {
		cmdArgs = append(cmdArgs, "--", args.Filter)
	}

	// Compile the filter first so that a bad interface or filter gets reported.
	_, err := subprocess.RunCommandContext(r.Context(), "tcpdump", append([]string{"-d"}, cmdArgs...)...)
	if err != nil {
		_ = response.BadRequest(err).Render(w)

		return
	}

	ctx := r.Context()

	if args.Duration > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, time.Duration(args.Duration)*time.Second)
		defer cancel()
	}

	_ = debugNetworkStream(ctx, w, false, "tcpdump", cmdArgs...)
}
