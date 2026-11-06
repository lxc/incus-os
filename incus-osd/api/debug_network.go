package api

// DebugNetworkPing defines the arguments for a network ping request.
//
// swagger:model
type DebugNetworkPing struct {
	// Host name or address to ping.
	Target string `json:"target" yaml:"target"`

	// Address family to use, either "ipv4" or "ipv6", automatically selected if unset.
	Family string `json:"family,omitempty" yaml:"family,omitempty"`

	// Interface to send from, automatically selected if unset.
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// Don't resolve addresses to names.
	Numeric bool `json:"numeric,omitempty" yaml:"numeric,omitempty"`

	// Number of packets to send, defaults to 10 if unset, 0 pings until the request is canceled.
	Count int `json:"count" yaml:"count"`

	// Seconds to wait between packets.
	Interval float64 `json:"interval,omitempty" yaml:"interval,omitempty"`

	// Payload size in bytes.
	Size int `json:"size,omitempty" yaml:"size,omitempty"`

	// Path MTU discovery strategy, one of "do", "dont", "want" or "probe".
	PMTUDiscovery string `json:"pmtu_discovery,omitempty" yaml:"pmtu_discovery,omitempty"`
}

// DebugNetworkTrace defines the arguments for a network trace request.
//
// swagger:model
type DebugNetworkTrace struct {
	// Host name or address to trace the path to.
	Target string `json:"target" yaml:"target"`

	// Address family to use, either "ipv4" or "ipv6", automatically selected if unset.
	Family string `json:"family,omitempty" yaml:"family,omitempty"`

	// Interface to send from, automatically selected if unset.
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// Don't resolve addresses to names.
	Numeric bool `json:"numeric,omitempty" yaml:"numeric,omitempty"`

	// Number of probes to send to each hop, defaults to 10 if unset, 0 traces until the request is canceled.
	Count int `json:"count" yaml:"count"`

	// Output format, either "json" (default) or "text".
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
}

// DebugNetworkSpeedTest defines the arguments for a network speed test request.
//
// swagger:model
type DebugNetworkSpeedTest struct {
	// Host name or address of the iperf3 server.
	Target string `json:"target" yaml:"target"`

	// Port of the iperf3 server, defaults to 5201.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// Address family to use, either "ipv4" or "ipv6", automatically selected if unset.
	Family string `json:"family,omitempty" yaml:"family,omitempty"`

	// Interface to send from, automatically selected if unset.
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// Protocol to test with, either "tcp" (default) or "udp".
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`

	// Target bitrate in bits per second with an optional K, M or G suffix, 0 for unlimited.
	Bitrate string `json:"bitrate,omitempty" yaml:"bitrate,omitempty"`

	// Number of parallel connections to test with, defaults to 1.
	Parallel int `json:"parallel,omitempty" yaml:"parallel,omitempty"`

	// Seconds to test for, defaults to 10 if unset, 0 tests until the request is canceled.
	Duration int `json:"duration" yaml:"duration"`

	// Measure the download rather than upload bandwidth.
	Reverse bool `json:"reverse,omitempty" yaml:"reverse,omitempty"`

	// Output format, either "json" (default) or "text".
	Format string `json:"format,omitempty" yaml:"format,omitempty"`
}

// DebugNetworkDumpTraffic defines the arguments for a network traffic dump request.
//
// swagger:model
type DebugNetworkDumpTraffic struct {
	// Interface to capture on, defaults to all interfaces.
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// Capture filter expression in tcpdump syntax.
	Filter string `json:"filter,omitempty" yaml:"filter,omitempty"`

	// Output format, either "text" (default) or "pcap".
	Format string `json:"format,omitempty" yaml:"format,omitempty"`

	// Seconds to capture for, defaults to 10 if unset, 0 captures until the request is canceled.
	Duration int `json:"duration" yaml:"duration"`
}

// DebugNetworkRoutes holds the routing policy rules and tables as reported by iproute2.
//
// swagger:model
type DebugNetworkRoutes struct {
	IPv4Rules  []map[string]any `json:"ipv4_rules"  yaml:"ipv4_rules"`
	IPv4Routes []map[string]any `json:"ipv4_routes" yaml:"ipv4_routes"`
	IPv6Rules  []map[string]any `json:"ipv6_rules"  yaml:"ipv6_rules"`
	IPv6Routes []map[string]any `json:"ipv6_routes" yaml:"ipv6_routes"`
}
