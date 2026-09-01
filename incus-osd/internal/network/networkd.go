package network

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	ocapi "github.com/FuturFusion/operations-center/shared/api"
	"github.com/lxc/incus/v7/shared/subprocess"
	"github.com/lxc/incus/v7/shared/units"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/nftables"
	"github.com/lxc/incus-os/incus-osd/internal/proxy"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

//go:embed templates/*
var templates embed.FS

var muNetworkState sync.Mutex

// networkdConfigFile represents a given filename and its contents.
type networkdConfigFile struct {
	Name     string
	Contents string
}

// expPhysDev holds the name, underlying physical interface, and MAC of a network device.
type expPhysDev struct {
	Name      string
	Interface string
	Hwaddr    string
}

// ApplyNetworkConfiguration instructs systemd-networkd to apply the supplied network configuration.
func ApplyNetworkConfiguration(ctx context.Context, s *state.State, networkCfg *api.SystemNetworkConfig, timeout time.Duration, allowPartialConfig bool, refresh func(context.Context, *state.State, ocapi.ServerSelfUpdateCause) error, delayRefreshCheck bool) error {
	if s.System.Network.State.ConfigurationInProcess {
		return errors.New("a network configuration is already in progress")
	}

	// Indicate when network configuration has begun and concluded.
	s.System.Network.State.ConfigurationInProcess = true
	defer func() {
		s.System.Network.State.ConfigurationInProcess = false
	}()

	// If a timezone is specified, apply it before doing any network configuration.
	err := systemd.SetTimezone(ctx, networkCfg.Time)
	if err != nil {
		return err
	}

	// Validate the new network configuration, allowing for invalid MACs.
	err = ValidateNetworkConfiguration(networkCfg, false)
	if err != nil {
		return err
	}

	// Before proceeding, dynamically lookup any MAC address that is referred to by an interface name.
	// This could be the case when reading in seed data, or if a user provides an interface
	// name via an API update.
	err = resolveMACs(ctx, networkCfg)
	if err != nil {
		return err
	}

	// Now, perform a strict validation of the new network configuration before proceeding.
	err = ValidateNetworkConfiguration(networkCfg, true)
	if err != nil {
		return err
	}

	// Delete any interfaces, bonds, or vlans that currently exist but don't in
	// the new configuration, or have a different configuration.
	err = cleanupStaleDevices(ctx, s.System.Network.Config, networkCfg)
	if err != nil {
		return err
	}

	// Generate a new private key for each wireguard if none is given.
	err = checkWireguardPrivateKeys(ctx, networkCfg)
	if err != nil {
		return err
	}

	// Determine if any new physical devices (starting with "_p") will be added. Later
	// after generating the new network configuration files we will need to wait until
	// the new devices are properly renamed by udev.
	expectedNewPhysicalDevices := getExpectedNewPhysicalDevices(ctx, networkCfg)

	// Update the state before (re)generating networking configuration.
	s.System.Network.Config = networkCfg

	// Apply the configured hostname, or reset back to default if not set.
	err = systemd.SetHostname(ctx, s.Hostname())
	if err != nil {
		return err
	}

	// Configure and startup the local proxy daemon.
	err = proxy.StartLocalProxy(ctx, networkCfg.Proxy)
	if err != nil {
		return err
	}

	err = generateNetworkConfiguration(ctx, networkCfg)
	if err != nil {
		return err
	}

	err = generateHosts(ctx, s)
	if err != nil {
		return err
	}

	err = waitForUdevInterfaceRename(ctx, expectedNewPhysicalDevices, 5*time.Second)
	if err != nil {
		return err
	}

	// Apply the ingress firewall rules.
	err = nftables.ApplyInputFilters(ctx, networkCfg)
	if err != nil {
		return err
	}

	// Apply the forwarding firewall rules.
	err = nftables.ApplyForwardFilters(ctx, networkCfg)
	if err != nil {
		return err
	}

	// Apply the conntrack bypass rules.
	err = nftables.ApplyNotrackFilters(ctx, networkCfg)
	if err != nil {
		return err
	}

	// Restart networking after new config files have been generated.
	err = systemd.RestartUnit(ctx, "systemd-networkd")
	if err != nil {
		return err
	}

	// Wait for the network to apply.
	err = waitForNetworkOnline(ctx, networkCfg, timeout)
	if err != nil {
		return err
	}

	// Wait for DNS to be functional.
	err = waitForDNS(ctx, timeout)
	if err != nil {
		if !allowPartialConfig {
			return err
		}

		slog.WarnContext(ctx, "DNS check failed, system may have trouble resolving hostnames")
	}

	// (Re)start NTP time synchronization. Since we might be overriding the default fallback NTP servers,
	// the service is disabled by default and only started once we have performed the network (re)configuration.
	err = systemd.RestartUnit(ctx, "systemd-timesyncd")
	if err != nil {
		return err
	}

	// Wait up to 30 seconds for NTP synchronization, but don't fail if it doesn't happen.
	err = waitForSystemdTimesyncd(ctx, 30*time.Second)
	if err != nil {
		slog.WarnContext(ctx, "systemd-timesyncd failed to perform NTP synchronization, system time may be incorrect")
	}

	// Refresh the state struct.
	err = UpdateNetworkState(ctx, &s.System.Network)
	if err != nil {
		return err
	}

	// Refresh registration, delaying by 30 seconds if needed to allow the provider to become available,
	// such as when IncusOS is self-hosting Operations Center.
	if refresh != nil {
		// #nosec G118
		go func() { //nolint:contextcheck
			if delayRefreshCheck {
				time.Sleep(30 * time.Second)
			}

			ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ctxCancel()

			err := refresh(ctx, s, ocapi.ServerSelfUpdateCauseNetworkConfigChanged)
			if err != nil {
				slog.WarnContext(ctx, "Failed to refresh provider registration", "err", err)
			}
		}()
	}

	return nil
}

// ValidateNetworkConfiguration performs some basic validation checks on the supplied network configuration.
func ValidateNetworkConfiguration(networkCfg *api.SystemNetworkConfig, requireValidMAC bool) error {
	if networkCfg == nil {
		return errors.New("no network configuration provided")
	}

	// Check that all interface/bond/vlan names and MACs are unique.
	names := []string{}
	macs := []string{}

	for _, iface := range networkCfg.Interfaces {
		if slices.Contains(names, iface.Name) {
			return errors.New("duplicate interface/bond/vlan/wireguard name: " + iface.Name)
		}

		if slices.Contains(macs, iface.Hwaddr) {
			return errors.New("duplicate MAC address: " + iface.Hwaddr)
		}

		names = append(names, iface.Name)
		macs = append(macs, iface.Hwaddr)
	}

	for _, bond := range networkCfg.Bonds {
		if slices.Contains(names, bond.Name) {
			return errors.New("duplicate interface/bond/vlan/wireguard name: " + bond.Name)
		}

		names = append(names, bond.Name)

		// A bond configuration may not explicitly define a MAC, so only check if one is present.
		if bond.Hwaddr != "" {
			if slices.Contains(macs, bond.Hwaddr) {
				return errors.New("duplicate MAC address: " + bond.Hwaddr)
			}

			macs = append(macs, bond.Hwaddr)
		}

		// Check that each bond member has a unique MAC, with the exception that the
		// bond may be configured with the same MAC as one of its members.
		numMembersWithBondMac := 0

		for _, memberMAC := range bond.Members {
			if memberMAC == bond.Hwaddr {
				numMembersWithBondMac++

				if numMembersWithBondMac > 1 {
					return errors.New("duplicate MAC address: " + memberMAC)
				}
			} else {
				if slices.Contains(macs, memberMAC) {
					return errors.New("duplicate MAC address: " + memberMAC)
				}

				macs = append(macs, memberMAC)
			}
		}
	}

	for _, vlan := range networkCfg.VLANs {
		if slices.Contains(names, vlan.Name) {
			return errors.New("duplicate interface/bond/vlan/wireguard name: " + vlan.Name)
		}

		names = append(names, vlan.Name)
	}

	for _, wg := range networkCfg.Wireguard {
		if slices.Contains(names, wg.Name) {
			return errors.New("duplicate interface/bond/vlan/wireguard name: " + wg.Name)
		}

		names = append(names, wg.Name)
	}

	// Some USB NICs have a default name of "enx<MAC>", which is 15 characters long.
	// To work around this, strip the leading "enx" before validating network interfaces.
	mangleUSBNICs(networkCfg)

	err := validateInterfaces(networkCfg.Interfaces, requireValidMAC)
	if err != nil {
		return err
	}

	err = validateBonds(networkCfg.Bonds, requireValidMAC)
	if err != nil {
		return err
	}

	err = validateVLANs(networkCfg)
	if err != nil {
		return err
	}

	err = validateWireguard(networkCfg)
	if err != nil {
		return err
	}

	return nil
}

// UpdateNetworkState updates the network state within the SystemNetwork struct.
func UpdateNetworkState(ctx context.Context, n *api.SystemNetwork) error {
	var err error

	// Prevent concurrent updates to the state information.
	muNetworkState.Lock()
	defer muNetworkState.Unlock()

	if n.Config == nil {
		return errors.New("no network configuration defined")
	}

	// Clear any existing state.
	n.State = api.SystemNetworkState{
		Interfaces: make(map[string]api.SystemNetworkInterfaceState),
	}

	// Keep track of all the roles being applied.
	rolesFound := []string{}

	// State update for interfaces.
	for _, i := range n.Config.Interfaces {
		iState, err := getInterfaceState(ctx, "interface", i.Name, i.Hwaddr, "", nil)
		if err != nil {
			return err
		}

		iState.Roles = i.Roles
		rolesFound = append(rolesFound, i.Roles...)
		n.State.Interfaces[i.Name] = iState
	}

	// State update for bonds.
	for _, b := range n.Config.Bonds {
		members := make(map[string]api.SystemNetworkInterfaceState)

		for _, m := range b.Members {
			mName := "_p" + strings.ToLower(strings.ReplaceAll(m, ":", ""))

			members[mName], err = getInterfaceState(ctx, "bond_member", mName, m, "", nil)
			if err != nil {
				return err
			}
		}

		bState, err := getInterfaceState(ctx, "bond", b.Name, b.Hwaddr, "", members)
		if err != nil {
			return err
		}

		bState.Roles = b.Roles
		rolesFound = append(rolesFound, b.Roles...)
		n.State.Interfaces[b.Name] = bState
	}

	// State update for vlans.
	for _, v := range n.Config.VLANs {
		hwaddr := ""

		parent, ok := n.State.Interfaces[v.Parent]
		if ok {
			hwaddr = parent.Hwaddr
		}

		vState, err := getInterfaceState(ctx, "vlan", v.Name, hwaddr, v.Parent, nil)
		if err != nil {
			return err
		}

		vState.Roles = v.Roles
		rolesFound = append(rolesFound, v.Roles...)
		n.State.Interfaces[v.Name] = vState
	}

	// State update for wireguard.
	for _, wg := range n.Config.Wireguard {
		wgState, err := getWireguardState(ctx, wg.Name)
		if err != nil {
			return err
		}

		wgState.Roles = wg.Roles
		rolesFound = append(rolesFound, wg.Roles...)
		n.State.Interfaces[wg.Name] = wgState
	}

	// Ensure required roles exist.
	if !slices.Contains(rolesFound, api.SystemNetworkInterfaceRoleManagement) || !slices.Contains(rolesFound, api.SystemNetworkInterfaceRoleCluster) {
		for iName, i := range n.State.Interfaces {
			iState := i

			if !slices.Contains(rolesFound, api.SystemNetworkInterfaceRoleManagement) && iState.State == "routable" {
				if iState.Roles == nil {
					iState.Roles = []string{}
				}

				iState.Roles = append(iState.Roles, api.SystemNetworkInterfaceRoleManagement)
			}

			if !slices.Contains(rolesFound, api.SystemNetworkInterfaceRoleCluster) && slices.Contains(iState.Roles, api.SystemNetworkInterfaceRoleManagement) {
				iState.Roles = append(iState.Roles, api.SystemNetworkInterfaceRoleCluster)
			}

			n.State.Interfaces[iName] = iState
		}
	}

	// Report any unused additional physical interface.
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, i := range ifaces {
		// Managed interfaces.
		if strings.HasPrefix(i.Name, "_") {
			continue
		}

		// Internal OS interfaces.
		if slices.Contains([]string{"lo"}, i.Name) {
			continue
		}

		// Already included in state.
		_, exists := n.State.Interfaces[i.Name]
		if exists {
			continue
		}

		// Check that it's not a virtual device.
		target, err := os.Readlink("/sys/class/net/" + i.Name)
		if err != nil {
			return err
		}

		if strings.Contains(target, "/virtual/") {
			continue
		}

		// Generate an entry.
		iState, err := getInterfaceState(ctx, "physical", i.Name, i.HardwareAddr.String(), "", nil)
		if err != nil {
			return err
		}

		n.State.Interfaces[i.Name] = iState
	}

	return nil
}

// RestoreWOLMACAddresses attempts to restore the permanent MAC address on any interface with Wake on LAN enabled.
// This gets called on system shutdown only.
func RestoreWOLMACAddresses(ctx context.Context, s *state.State) {
	restoreMAC := func(iface string, hwaddr string) error {
		// Set the MAC address.
		_, err := subprocess.RunCommandContext(ctx, "ip", "link", "set", "dev", iface, "address", hwaddr)
		if err != nil {
			return err
		}

		return nil
	}

	for _, i := range s.System.Network.Config.Interfaces {
		if i.Ethernet == nil || !i.Ethernet.WakeOnLAN {
			continue
		}

		iface := "_p" + strings.ToLower(strings.ReplaceAll(i.Hwaddr, ":", ""))

		err := restoreMAC(iface, i.Hwaddr)
		if err != nil {
			slog.Warn("Unable to restore MAC address", "interface", iface, "err", err)
		}
	}

	for _, b := range s.System.Network.Config.Bonds {
		if b.Ethernet == nil || !b.Ethernet.WakeOnLAN {
			continue
		}

		for _, hwaddr := range b.Members {
			iface := "_p" + strings.ToLower(strings.ReplaceAll(hwaddr, ":", ""))

			err := restoreMAC(iface, hwaddr)
			if err != nil {
				slog.Warn("Unable to restore MAC address", "interface", iface, "err", err)
			}
		}
	}
}

// getWireguardState runs various commands to gather wireguard state for a specific wireguard interface.
func getWireguardState(ctx context.Context, iface string) (api.SystemNetworkInterfaceState, error) {
	// Get IPs for the interface.
	ips, err := GetIPAddresses(ctx, iface)
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	// Get routes for the interface.
	routes := []api.SystemNetworkRoute{}
	routeRegex := regexp.MustCompile(`(.+) via (.+) proto`)

	output, err := subprocess.RunCommandContext(ctx, "ip", "route", "show", "dev", resolveBridge(iface))
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	for _, r := range routeRegex.FindAllStringSubmatch(output, -1) {
		routes = append(routes, api.SystemNetworkRoute{
			To:  r[1],
			Via: r[2],
		})
	}

	// Get various details from networkctl. It would be better to use the json output
	// option, but that doesn't include everything we're interested in.
	output, err = subprocess.RunCommandContext(ctx, "networkctl", "status", "-s", resolveBridge(iface))
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	interfaceStateRegex := regexp.MustCompile(`State: (.+?) `)

	interfaceState := ""
	if len(interfaceStateRegex.FindStringSubmatch(output)) == 2 {
		interfaceState = interfaceStateRegex.FindStringSubmatch(output)[1]
	}

	mtuRegex := regexp.MustCompile(`MTU: (.+?) `)

	mtu, err := strconv.Atoi(mtuRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	rxBytesRegex := regexp.MustCompile(`Rx Bytes: (.+)`)

	rxBytes, err := strconv.Atoi(rxBytesRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	txBytesRegex := regexp.MustCompile(`Tx Bytes: (.+)`)

	txBytes, err := strconv.Atoi(txBytesRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	rxErrorsRegex := regexp.MustCompile(`Rx Errors: (.+)`)

	rxErrors, err := strconv.Atoi(rxErrorsRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	txErrorsRegex := regexp.MustCompile(`Tx Errors: (.+)`)

	txErrors, err := strconv.Atoi(txErrorsRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	var (
		publicKey     string
		listeningPort int
		wgSections    []string
	)

	peers := []api.SystemNetworkWireguardPeerState{}

	if interfaceState != "off" {
		// Get various details from wg show output.
		output, err = subprocess.RunCommandContext(ctx, "wg", "show", resolveBridge(iface))
		if err != nil {
			return api.SystemNetworkInterfaceState{}, err
		}

		wgSections = strings.Split(output, "\n\n")
	}

	// Loop over sections of wg show and fill stats.
	for sectionIndex, section := range wgSections {
		if sectionIndex == 0 { //nolint:nestif
			publicKeyRegex := regexp.MustCompile(`  public key: (.+)`)
			content := publicKeyRegex.FindStringSubmatch(section)[1]
			publicKey = strings.TrimSuffix(content, "\n")

			listeningPortRegex := regexp.MustCompile(`  listening port: (.+)`)

			port, err := strconv.Atoi(listeningPortRegex.FindStringSubmatch(section)[1])
			if err != nil {
				return api.SystemNetworkInterfaceState{}, err
			}

			listeningPort = port
		} else {
			pubKeyRegex := regexp.MustCompile(`peer: (.+)`)

			pubKey := ""
			if len(pubKeyRegex.FindStringSubmatch(section)) == 2 {
				pubKey = pubKeyRegex.FindStringSubmatch(section)[1]
			}

			endPointRegex := regexp.MustCompile(`  endpoint: (.+)`)

			endPoint := ""
			if len(endPointRegex.FindStringSubmatch(section)) == 2 {
				endPoint = endPointRegex.FindStringSubmatch(section)[1]
			}

			handshakeRegex := regexp.MustCompile(`  latest handshake: (.+)`)

			handshake := ""
			if len(handshakeRegex.FindStringSubmatch(section)) == 2 {
				handshake = handshakeRegex.FindStringSubmatch(section)[1]
			}

			addressesRegex := regexp.MustCompile(`  allowed ips: (.+)`)
			addresses := strings.Split(strings.ReplaceAll(addressesRegex.FindStringSubmatch(section)[1], " ", ""), ",")

			transferRegex := regexp.MustCompile(`  transfer: (.+) received, (.+) sent`)
			transfer := transferRegex.FindStringSubmatch(section)

			rxStats := 0
			txStats := 0

			if transfer != nil {
				cleanValue := func(val string) string {
					var (
						fraction bool
						cleanVal strings.Builder
					)

					// Strip spaces and any fraction before handing it over to the Incus units parser.
					for _, chr := range []byte(val) {
						// Strip spaces.
						if chr == ' ' {
							continue
						}

						// Detect fractions.
						if chr == '.' {
							fraction = true

							continue
						}

						// Skip any integer after a fraction.
						_, err := strconv.Atoi(string([]byte{chr}))
						if err == nil && fraction {
							continue
						}

						// Keep the rest (leading integers and suffix).
						_, _ = cleanVal.Write([]byte{chr})
					}

					return cleanVal.String()
				}

				output, err := units.ParseByteSizeString(cleanValue(transfer[1]))
				if err != nil {
					return api.SystemNetworkInterfaceState{}, err
				}

				rxStats = int(output)

				output, err = units.ParseByteSizeString(cleanValue(transfer[2]))
				if err != nil {
					return api.SystemNetworkInterfaceState{}, err
				}

				txStats = int(output)
			}

			peers = append(peers, api.SystemNetworkWireguardPeerState{
				AllowedIPs:      addresses,
				EndPoint:        endPoint,
				LatestHandshake: handshake,
				PublicKey:       pubKey,
				Stats: api.SystemNetworkInterfaceStats{
					RXBytes:  rxStats,
					RXErrors: 0,
					TXBytes:  txStats,
					TXErrors: 0,
				},
			})
		}
	}

	return api.SystemNetworkInterfaceState{
		Type:      "wireguard",
		Addresses: ips,
		Routes:    routes,
		MTU:       mtu,
		Speed:     "unknown",
		State:     interfaceState,
		Stats: api.SystemNetworkInterfaceStats{
			RXBytes:  rxBytes,
			TXBytes:  txBytes,
			RXErrors: rxErrors,
			TXErrors: txErrors,
		},
		Wireguard: &api.SystemNetworkWireguardState{
			ListeningPort: listeningPort,
			PublicKey:     publicKey,
			Peers:         peers,
		},
	}, nil
}

// getInterfaceState runs various commands to gather network state for a specific interface.
func getInterfaceState(ctx context.Context, ifaceType string, iface string, hwaddr string, parent string, members map[string]api.SystemNetworkInterfaceState) (api.SystemNetworkInterfaceState, error) {
	// Get IPs for the interface.
	ips, err := GetIPAddresses(ctx, iface)
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	// Get routes for the interface.
	routes := []api.SystemNetworkRoute{}
	routeRegex := regexp.MustCompile(`(.+) via (.+) proto`)

	output, err := subprocess.RunCommandContext(ctx, "ip", "route", "show", "dev", resolveBridge(iface))
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	for _, r := range routeRegex.FindAllStringSubmatch(output, -1) {
		routes = append(routes, api.SystemNetworkRoute{
			To:  r[1],
			Via: r[2],
		})
	}

	// Get various details from networkctl. It would be better to use the json output
	// option, but that doesn't include everything we're interested in.
	output, err = subprocess.RunCommandContext(ctx, "networkctl", "status", "-s", resolveBridge(iface))
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	interfaceStateRegex := regexp.MustCompile(`State: (.+?) `)

	interfaceState := ""
	if len(interfaceStateRegex.FindStringSubmatch(output)) == 2 {
		interfaceState = interfaceStateRegex.FindStringSubmatch(output)[1]
	}

	localMACRegex := regexp.MustCompile(`  Hardware Address: (.+)`)

	localMAC := ""
	if len(localMACRegex.FindStringSubmatch(output)) == 2 {
		localMAC = strings.Fields(localMACRegex.FindStringSubmatch(output)[1])[0]
	}

	remoteMACRegex := regexp.MustCompile(`Permanent Hardware Address: (.+)`)

	remoteMAC := ""
	if len(remoteMACRegex.FindStringSubmatch(output)) == 2 {
		remoteMAC = strings.Fields(remoteMACRegex.FindStringSubmatch(output)[1])[0]
	}

	mtuRegex := regexp.MustCompile(`MTU: (.+?) `)

	mtu, err := strconv.Atoi(mtuRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	rxBytesRegex := regexp.MustCompile(`Rx Bytes: (.+)`)

	rxBytes, err := strconv.Atoi(rxBytesRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	txBytesRegex := regexp.MustCompile(`Tx Bytes: (.+)`)

	txBytes, err := strconv.Atoi(txBytesRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	rxErrorsRegex := regexp.MustCompile(`Rx Errors: (.+)`)

	rxErrors, err := strconv.Atoi(rxErrorsRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	txErrorsRegex := regexp.MustCompile(`Tx Errors: (.+)`)

	txErrors, err := strconv.Atoi(txErrorsRegex.FindStringSubmatch(output)[1])
	if err != nil {
		return api.SystemNetworkInterfaceState{}, err
	}

	// Get the actual underlying device's speed; querying the veth device always
	// returns 10Gbps. Interfaces, bond members, and vlans directly on an interface
	// look at the actual device, while bonds and vlans on a bond look at the bond
	// device.
	var underlyingDevice string

	switch ifaceType {
	case "interface", "bond_member":
		underlyingDevice = "_p" + strings.ToLower(strings.ReplaceAll(hwaddr, ":", ""))
	case "bond", "physical":
		underlyingDevice = iface
	case "vlan":
		if hwaddr == "" {
			underlyingDevice = parent
		} else {
			underlyingDevice = "_p" + strings.ToLower(strings.ReplaceAll(hwaddr, ":", ""))
		}
	default:
		return api.SystemNetworkInterfaceState{}, errors.New("unknown interface type '" + ifaceType + "' for interface " + iface)
	}

	var speed string

	if interfaceState != "off" {
		// #nosec G304
		contents, err := os.ReadFile("/sys/class/net/" + underlyingDevice + "/speed")
		if err != nil {
			speed = "unknown"
		} else {
			speed = strings.TrimSuffix(string(contents), "\n")
		}
	}

	// Fetch any LLDP info.
	lldp := []api.SystemNetworkLLDPState{}

	if ifaceType == "interface" || ifaceType == "bond_member" {
		lldpIface := iface
		if ifaceType == "interface" {
			lldpIface = "_p" + strings.ToLower(strings.ReplaceAll(localMAC, ":", ""))
		}

		lldp, err = getLLDPInfo(ctx, lldpIface)
		if err != nil {
			return api.SystemNetworkInterfaceState{}, err
		}
	}

	// Get LACP info for a bond member.
	var lacp *api.SystemNetworkLACPState
	if ifaceType == "bond_member" {
		lacp = &api.SystemNetworkLACPState{
			LocalMAC:  localMAC,
			RemoteMAC: remoteMAC,
		}
	}

	return api.SystemNetworkInterfaceState{
		Type:      ifaceType,
		Addresses: ips,
		Hwaddr:    hwaddr,
		Routes:    routes,
		MTU:       mtu,
		Speed:     speed,
		State:     interfaceState,
		Stats: api.SystemNetworkInterfaceStats{
			RXBytes:  rxBytes,
			TXBytes:  txBytes,
			RXErrors: rxErrors,
			TXErrors: txErrors,
		},
		LLDP:    lldp,
		LACP:    lacp,
		Members: members,
	}, nil
}

// When dealing with a bridge, we can't just get its IP address or route. So,
// determine the "main" member corresponding to the physical NIC and return that
// device name instead. If the device isn't a bridge, return the original name
// unmodified.
func resolveBridge(iface string) string {
	_, err := os.ReadDir("/sys/class/net/" + iface + "/brif")
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return iface
	}

	return "_v" + iface
}

// GetIPAddresses returns any non-link-local address for an interface.
func GetIPAddresses(ctx context.Context, iface string) ([]string, error) {
	ipAddressRegex := regexp.MustCompile(`inet6? (.+)/\d+ `)
	dev := resolveBridge(iface)

	output, err := subprocess.RunCommandContext(ctx, "ip", "address", "show", dev)
	if err != nil {
		// We typically expect to only attempt to get IP address(es) for interfaces
		// that exist, but if the user is in the middle of editing their configuration
		// we might be in an inconstant state. Rather than returning an error and
		// preventing the user from being able to correct things, log a warning and
		// return no IP addresses.
		if strings.Contains(err.Error(), `Device "`+dev+`" does not exist.`) {
			slog.WarnContext(ctx, "Failed to get IP address(es) for "+dev)

			return nil, nil
		}

		return nil, err
	}

	ret := []string{}
	matches := ipAddressRegex.FindAllStringSubmatch(output, -1)

	for _, addr := range matches {
		// Don't count link-local addresses.
		if strings.HasPrefix(addr[1], "169.254.") || strings.HasPrefix(addr[1], "fe80:") {
			continue
		}

		ret = append(ret, addr[1])
	}

	return ret, nil
}

// getLLDPInfo returns current LLDP information for the interface's underlying physical device.
func getLLDPInfo(ctx context.Context, iface string) ([]api.SystemNetworkLLDPState, error) {
	output, err := subprocess.RunCommandContext(ctx, "networkctl", "lldp", "--json=short", resolveBridge(iface))
	if err != nil {
		return nil, err
	}

	type lldpStruct struct {
		Neighbors []struct {
			InterfaceName string `json:"InterfaceName"` //nolint:tagliatelle
			Neighbors     []struct {
				SystemName      string `json:"SystemName"`      //nolint:tagliatelle
				ChassisID       string `json:"ChassisID"`       //nolint:tagliatelle
				PortID          string `json:"PortID"`          //nolint:tagliatelle
				PortDescription string `json:"PortDescription"` //nolint:tagliatelle
			} `json:"Neighbors"` //nolint:tagliatelle
		} `json:"Neighbors"` //nolint:tagliatelle
	}

	lldp := lldpStruct{}

	err = json.Unmarshal([]byte(output), &lldp)
	if err != nil {
		return nil, err
	}

	if len(lldp.Neighbors) == 0 {
		return nil, nil
	}

	ret := []api.SystemNetworkLLDPState{}
	for _, n := range lldp.Neighbors[0].Neighbors {
		ret = append(ret, api.SystemNetworkLLDPState{
			Name:      n.SystemName,
			ChassisID: n.ChassisID,
			PortID:    n.PortID,
			Port:      n.PortDescription,
		})
	}

	return ret, nil
}

func generateHosts(_ context.Context, s *state.State) error {
	// Generate the /etc/hosts file.
	return os.WriteFile("/etc/hosts", fmt.Appendf([]byte{}, `127.0.0.1	localhost
127.0.1.1	%s

# The following lines are desirable for IPv6 capable hosts
::1     localhost ip6-localhost ip6-loopback
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
`, s.Hostname()), 0o644)
}

// generateNetworkConfiguration clears any existing configuration from /run/systemd/network/ and generates
// new config files from the supplied NetworkConfig struct.
func generateNetworkConfiguration(ctx context.Context, networkCfg *api.SystemNetworkConfig) error {
	// Remove any existing configuration.
	err := os.RemoveAll(systemd.SystemdNetworkConfigPath)
	if err != nil {
		return err
	}

	err = os.Mkdir(systemd.SystemdNetworkConfigPath, 0o755)
	if err != nil {
		return err
	}

	// Generate .link files.
	cfgs, err := generateLinkFileContents(ctx, *networkCfg)
	if err != nil {
		return err
	}

	for _, cfg := range cfgs {
		err := os.WriteFile(filepath.Join(systemd.SystemdNetworkConfigPath, cfg.Name), []byte(cfg.Contents), 0o644)
		if err != nil {
			return err
		}
	}

	// Generate .netdev files.
	cfgs, err = generateNetdevFileContents(*networkCfg)
	if err != nil {
		return err
	}

	for _, cfg := range cfgs {
		err := os.WriteFile(filepath.Join(systemd.SystemdNetworkConfigPath, cfg.Name), []byte(cfg.Contents), 0o644)
		if err != nil {
			return err
		}
	}

	// Generate .network files.
	cfgs, err = generateNetworkFileContents(ctx, *networkCfg)
	if err != nil {
		return err
	}

	for _, cfg := range cfgs {
		err := os.WriteFile(filepath.Join(systemd.SystemdNetworkConfigPath, cfg.Name), []byte(cfg.Contents), 0o644)
		if err != nil {
			return err
		}
	}

	// Generate systemd-timesyncd configuration if any timeservers are defined.
	if networkCfg.Time != nil && len(networkCfg.Time.NTPServers) > 0 {
		ntpCfg := "[Time]\nFallbackNTP=" + strings.Join(networkCfg.Time.NTPServers, " ") + "\n"

		err := os.WriteFile(systemd.SystemdTimesyncConfigFile, []byte(ntpCfg), 0o644)
		if err != nil {
			return err
		}
	} else {
		// If there's no NTP configuration, remove any old config file that might exist.
		_ = os.Remove(systemd.SystemdTimesyncConfigFile)
	}

	return nil
}

// waitForUdevInterfaceRename waits up to a provided timeout for udev to pickup and process
// the renaming of interfaces. At system startup there's a small race between udev being fully
// started and our reconfiguring of the network, so we poll in a loop until we see the kernel
// has been notified of the rename.
func waitForUdevInterfaceRename(ctx context.Context, expectedInterfaces []expPhysDev, timeout time.Duration) error {
	endTime := time.Now().Add(timeout)
	missingInterfaces := []expPhysDev{}

	for {
		if time.Now().After(endTime) {
			missing := make([]string, 0, len(missingInterfaces))

			for _, dev := range missingInterfaces {
				missing = append(missing, dev.Name+" ("+dev.Hwaddr+")")
			}

			return errors.New("timed out waiting for configured network interfaces, missing interface(s): " + strings.Join(missing, ", "))
		}

		// Trigger udev rule update to pickup device names.
		_, err := subprocess.RunCommandContext(ctx, "udevadm", "trigger", "--action=add", "--subsystem-match=net")
		if err != nil {
			return err
		}

		// Wait for udev to be done processing the events.
		_, err = subprocess.RunCommandContext(ctx, "udevadm", "settle")
		if err != nil {
			return err
		}

		missingInterfaces = []expPhysDev{}

		// Check if the kernel has noticed the renaming of each of the expected
		// interfaces to the "_p<MAC address>" format.
		for _, dev := range expectedInterfaces {
			_, err = subprocess.RunCommandContext(ctx, "journalctl", "--since", "10 seconds ago", "-t", "kernel", "-g", dev.Interface+": renamed from ")
			if err != nil {
				missingInterfaces = append(missingInterfaces, dev)
			}
		}

		if len(missingInterfaces) == 0 {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// waitForNetworkOnline waits up to a provided timeout for configured network interfaces,
// bonds, and vlans to configure their IP address(es) and come online.
func waitForNetworkOnline(ctx context.Context, networkCfg *api.SystemNetworkConfig, timeout time.Duration) error {
	isOnline := func(name string) (bool, bool) {
		output, err := subprocess.RunCommandContext(ctx, "networkctl", "status", resolveBridge(name))
		if err != nil {
			return false, true
		}

		return strings.Contains(output, "Online state: online"), strings.Contains(output, "Required For Online: yes")
	}

	hasAtLeastOneConfiguredIP := func(iface string) bool {
		ips, _ := GetIPAddresses(ctx, iface)

		return len(ips) > 0
	}

	endTime := time.Now().Add(timeout)

	devicesToCheck := []string{}

	needIPv6Delay := false

	for _, i := range networkCfg.Interfaces {
		if len(i.Addresses) == 0 {
			continue
		}

		if slices.Contains([]string{"ipv6", "both"}, i.RequiredForOnline) {
			needIPv6Delay = true
		}

		devicesToCheck = append(devicesToCheck, i.Name)
	}

	for _, b := range networkCfg.Bonds {
		if len(b.Addresses) == 0 {
			continue
		}

		if slices.Contains([]string{"ipv6", "both"}, b.RequiredForOnline) {
			needIPv6Delay = true
		}

		devicesToCheck = append(devicesToCheck, b.Name)
	}

	for _, v := range networkCfg.VLANs {
		if len(v.Addresses) == 0 {
			continue
		}

		if slices.Contains([]string{"ipv6", "both"}, v.RequiredForOnline) {
			needIPv6Delay = true
		}

		devicesToCheck = append(devicesToCheck, v.Name)
	}

	for {
		if time.Now().After(endTime) {
			return errors.New("timed out waiting for network to come online")
		}

		allDevicesOnline := true

		for _, name := range devicesToCheck {
			online, requiredOnline := isOnline(name)
			if !requiredOnline {
				continue
			}

			if !online || !hasAtLeastOneConfiguredIP(name) {
				allDevicesOnline = false

				break
			}
		}

		if allDevicesOnline {
			if needIPv6Delay {
				// Even with the interface configured to require IPv6
				// family connectivity, networkd will sometimes mark the interface as
				// online when IPv6 duplicate address detection is still running.
				//
				// This can lead to connectivity issues when IPv6 is
				// required, so add a 3s delay for DAD and related logic to complete.
				time.Sleep(3 * time.Second)
			}

			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// waitForDNS waits up to a provided timeout for the system to be able to resolve DNS records.
func waitForDNS(ctx context.Context, timeout time.Duration) error {
	endTime := time.Now().Add(timeout)
	resolver := net.Resolver{}

	for {
		if time.Now().After(endTime) {
			return errors.New("timed out waiting for DNS to respond")
		}

		// Attempt to resolve linuxcontainers.org to see if the DNS server is functional.
		_, err := resolver.LookupIPAddr(ctx, "linuxcontainers.org")
		if err == nil {
			// Valid response received from the DNS server.
			return nil
		}

		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			// NXDOMAIN received from the DNS server.
			// We still consider this as functional as we are talking to a live (though likely isolated) DNS server.
			return nil
		}

		if strings.HasSuffix(err.Error(), ": server misbehaving") {
			// REFUSED received from the DNS server.
			// This won't trigger in case of network connection errors (ICMP reject, broken route or timeout).
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// waitForSystemdTimesyncd waits up to a provided timeout for systemd-timesyncd to
// perform an initial NTP synchronization.
func waitForSystemdTimesyncd(ctx context.Context, timeout time.Duration) error {
	endTime := time.Now().Add(timeout)

	count := 0

	for {
		if time.Now().After(endTime) {
			return errors.New("timed out waiting for NTP synchronization")
		}

		// Check if systemd-timesyncd has performed its initial synchronization.
		_, err := subprocess.RunCommandContext(ctx, "journalctl", "--since", "10 seconds ago", "-u", "systemd-timesyncd", "-g", "Initial clock synchronization")
		if err == nil {
			return nil
		}

		// Restart systemd-timesyncd every 5 tries.
		count++
		if count == 5 {
			count = 0

			err = systemd.RestartUnit(ctx, "systemd-timesyncd")
			if err != nil {
				return err
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// generateLinkFileContents generates the contents of systemd.link files. Returns an array of ConfigFile structs.
// https://www.freedesktop.org/software/systemd/man/latest/systemd.link.html
func generateLinkFileContents(ctx context.Context, networkCfg api.SystemNetworkConfig) ([]networkdConfigFile, error) {
	t, err := template.ParseFS(templates, "templates/link.txt")
	if err != nil {
		return nil, err
	}

	ret := []networkdConfigFile{}

	var buf bytes.Buffer

	for _, i := range networkCfg.Interfaces {
		maxMTU, err := getMaxMTUForMAC(ctx, i.Hwaddr)
		if err != nil {
			return nil, err
		}

		strippedHwaddr := strings.ToLower(strings.ReplaceAll(i.Hwaddr, ":", ""))

		buf.Reset()

		err = t.Execute(&buf, linkFileVariables{
			Hwaddr:    i.Hwaddr,
			RandomMAC: true,
			Name:      strippedHwaddr,
			MTU:       maxMTU,
			Ethernet:  i.Ethernet,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("00-_p%s.link", strippedHwaddr),
			Contents: buf.String(),
		})
	}

	for _, b := range networkCfg.Bonds {
		for _, member := range b.Members {
			maxMTU, err := getMaxMTUForMAC(ctx, member)
			if err != nil {
				return nil, err
			}

			strippedHwaddr := strings.ToLower(strings.ReplaceAll(member, ":", ""))

			buf.Reset()

			err = t.Execute(&buf, linkFileVariables{
				Hwaddr:    member,
				RandomMAC: false,
				Name:      strippedHwaddr,
				MTU:       maxMTU,
				Ethernet:  b.Ethernet,
			})
			if err != nil {
				return nil, err
			}

			ret = append(ret, networkdConfigFile{
				Name:     fmt.Sprintf("01-_p%s.link", strippedHwaddr),
				Contents: buf.String(),
			})
		}
	}

	return ret, nil
}

// generateNetdevFileContents generates the contents of systemd.netdev files. Returns an array of networkdConfigFile structs.
// https://www.freedesktop.org/software/systemd/man/latest/systemd.netdev.html
func generateNetdevFileContents(networkCfg api.SystemNetworkConfig) ([]networkdConfigFile, error) {
	t, err := template.ParseFS(templates, "templates/netdev.txt")
	if err != nil {
		return nil, err
	}

	ret := make([]networkdConfigFile, 0, 2*len(networkCfg.Interfaces)+3*len(networkCfg.Bonds)+len(networkCfg.Wireguard))

	var buf bytes.Buffer

	// Create bridge and veth devices for each interface.
	for _, i := range networkCfg.Interfaces {
		// Bridge.
		buf.Reset()

		err := t.Execute(&buf, netdevFileVariables{
			Type:   "bridge",
			Name:   i.Name,
			Bridge: i.Bridge,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("10-%s.netdev", i.Name),
			Contents: buf.String(),
		})

		// veth.
		buf.Reset()

		err = t.Execute(&buf, netdevFileVariables{
			Type:           "veth",
			Name:           "_v" + i.Name,
			Hwaddr:         i.Hwaddr,
			StrippedHwaddr: strings.ToLower(strings.ReplaceAll(i.Hwaddr, ":", "")),
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("10-_v%s.netdev", i.Name),
			Contents: buf.String(),
		})
	}

	// Create bond, bridge, and veth devices for each bond.
	for _, b := range networkCfg.Bonds {
		// Bond.
		buf.Reset()

		// Work on a copy of the options so defaults don't leak into the config.
		options := api.SystemNetworkBondOptions{}
		if b.Options != nil {
			options = *b.Options
		}

		if b.Mode == "802.3ad" {
			if options.TransmitHashPolicy == "" {
				options.TransmitHashPolicy = "layer3+4"
			}

			if options.LACPRate == "" {
				options.LACPRate = "fast"
			}
		}

		// Link monitoring, defaulting to MII monitoring at 100ms unless ARP monitoring is configured.
		if options.ARPInterval == 0 && options.MIIMonitorInterval == 0 {
			options.MIIMonitorInterval = 100
		}

		err := t.Execute(&buf, netdevFileVariables{
			Type:        "bond",
			Name:        "_b" + b.Name,
			BondMode:    b.Mode,
			BondOptions: &options,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("11-_b%s.netdev", b.Name),
			Contents: buf.String(),
		})

		// Bridge.
		buf.Reset()

		err = t.Execute(&buf, netdevFileVariables{
			Type:   "bridge",
			Name:   b.Name,
			Bridge: b.Bridge,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("11-%s.netdev", b.Name),
			Contents: buf.String(),
		})

		// veth.
		buf.Reset()

		bondMacAddr := b.Hwaddr
		if bondMacAddr == "" {
			bondMacAddr = b.Members[0]
		}

		err = t.Execute(&buf, netdevFileVariables{
			Type:           "veth",
			Name:           "_v" + b.Name,
			Hwaddr:         bondMacAddr,
			StrippedHwaddr: strings.ToLower(strings.ReplaceAll(bondMacAddr, ":", "")),
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("11-_v%s.netdev", b.Name),
			Contents: buf.String(),
		})
	}

	// Create vlans.
	for _, v := range networkCfg.VLANs {
		buf.Reset()

		err := t.Execute(&buf, netdevFileVariables{
			Type:   "vlan",
			Name:   v.Name,
			VLANID: v.ID,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("12-%s.netdev", v.Name),
			Contents: buf.String(),
		})
	}

	// Create wireguard.
	for _, w := range networkCfg.Wireguard {
		buf.Reset()

		err := t.Execute(&buf, netdevFileVariables{
			Type:         "wireguard",
			Name:         w.Name,
			WGPrivateKey: w.PrivateKey,
			WGPort:       w.Port,
			WGPeers:      w.Peers,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("13-%s.netdev", w.Name),
			Contents: buf.String(),
		})
	}

	return ret, nil
}

// generateNetworkFileContents generates the contents of systemd.network files. Returns an array of networkdConfigFile structs.
// https://www.freedesktop.org/software/systemd/man/latest/systemd.network.html
func generateNetworkFileContents(ctx context.Context, networkCfg api.SystemNetworkConfig) ([]networkdConfigFile, error) { //nolint:revive
	// Common template used by veth-user, vlan, and wireguard.
	commonT, err := template.ParseFS(templates, "templates/network-common.txt")
	if err != nil {
		return nil, err
	}

	physicalT, err := template.ParseFS(templates, "templates/network-physical.txt")
	if err != nil {
		return nil, err
	}

	bondBridgeT, err := template.ParseFS(templates, "templates/network-bond-bridge.txt")
	if err != nil {
		return nil, err
	}

	ret := []networkdConfigFile{}

	var buf bytes.Buffer

	// Create networks for each interface and its bridge.
	for _, i := range networkCfg.Interfaces {
		maxMTU, err := getMaxMTUForMAC(ctx, i.Hwaddr)
		if err != nil {
			return nil, err
		}

		configuredMTU := i.MTU
		if configuredMTU == 0 {
			configuredMTU = 1500
		} else if configuredMTU > maxMTU {
			configuredMTU = maxMTU
		}

		// User side of veth device.
		buf.Reset()

		err = commonT.Execute(&buf, networkFileVariables{
			Type:              "veth-user",
			Name:              i.Name,
			RequiredForOnline: i.RequiredForOnline,
			MTU:               configuredMTU,
			VLANs:             networkCfg.VLANs,
			DNS:               networkCfg.DNS,
			TimeConfig:        networkCfg.Time,
			Addresses:         i.Addresses,
			Routes:            i.Routes,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("20-_v%s.network", i.Name),
			Contents: buf.String(),
		})

		// Bridge side of veth device.
		buf.Reset()

		strippedHwaddr := strings.ToLower(strings.ReplaceAll(i.Hwaddr, ":", ""))

		err = bondBridgeT.Execute(&buf, networkFileVariables{
			Type:     "veth-bridge",
			Name:     strippedHwaddr,
			MTU:      maxMTU,
			Bridge:   i.Name,
			VLANTags: getVLANTags(i.Name, i.VLANTags, networkCfg.VLANs),
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("20-_i%s.network", strippedHwaddr),
			Contents: buf.String(),
		})

		// Add underlying interface to bridge.
		buf.Reset()

		err = physicalT.Execute(&buf, networkFileVariables{
			Name:     strippedHwaddr,
			MTU:      maxMTU,
			Bridge:   i.Name,
			LLDP:     strconv.FormatBool(i.LLDP),
			VLANTags: getVLANTags(i.Name, i.VLANTags, networkCfg.VLANs),
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("20-_p%s.network", strippedHwaddr),
			Contents: buf.String(),
		})

		// Bridge.
		buf.Reset()

		err = bondBridgeT.Execute(&buf, networkFileVariables{
			Type: "bridge",
			Name: i.Name,
			MTU:  maxMTU,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("20-%s.network", i.Name),
			Contents: buf.String(),
		})
	}

	// Create networks for each bond, its member(s), and its bridge.
	for _, b := range networkCfg.Bonds {
		maxMTU := 9000

		// Bond members.
		for index, member := range b.Members {
			mtu, err := getMaxMTUForMAC(ctx, member)
			if err != nil {
				return nil, err
			}

			// Set the maximum MTU for the bond to be the smallest of the member's
			// maximum MTU.
			if maxMTU > mtu {
				maxMTU = mtu
			}

			memberStrippedHwaddr := strings.ToLower(strings.ReplaceAll(member, ":", ""))

			buf.Reset()

			err = physicalT.Execute(&buf, networkFileVariables{
				Name: memberStrippedHwaddr,
				MTU:  mtu,
				Bond: b.Name,
				LLDP: strconv.FormatBool(b.LLDP),
			})
			if err != nil {
				return nil, err
			}

			ret = append(ret, networkdConfigFile{
				Name:     fmt.Sprintf("21-_b%s-dev%d.network", b.Name, index),
				Contents: buf.String(),
			})
		}

		configuredMTU := b.MTU
		if configuredMTU == 0 {
			configuredMTU = 1500
		} else if configuredMTU > maxMTU {
			configuredMTU = maxMTU
		}

		// User side of veth device.
		buf.Reset()

		err := commonT.Execute(&buf, networkFileVariables{
			Type:              "veth-user",
			Name:              b.Name,
			RequiredForOnline: b.RequiredForOnline,
			MTU:               configuredMTU,
			VLANs:             networkCfg.VLANs,
			DNS:               networkCfg.DNS,
			TimeConfig:        networkCfg.Time,
			Addresses:         b.Addresses,
			Routes:            b.Routes,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("21-_v%s.network", b.Name),
			Contents: buf.String(),
		})

		// Bridge side of veth device.
		buf.Reset()

		bondMacAddr := b.Hwaddr
		if bondMacAddr == "" {
			bondMacAddr = b.Members[0]
		}

		strippedHwaddr := strings.ToLower(strings.ReplaceAll(bondMacAddr, ":", ""))

		err = bondBridgeT.Execute(&buf, networkFileVariables{
			Type:     "veth-bridge",
			Name:     strippedHwaddr,
			MTU:      maxMTU,
			Bridge:   b.Name,
			VLANTags: getVLANTags(b.Name, b.VLANTags, networkCfg.VLANs),
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("21-_i%s.network", strippedHwaddr),
			Contents: buf.String(),
		})

		// Add bond to bridge.
		buf.Reset()

		err = bondBridgeT.Execute(&buf, networkFileVariables{
			Type:     "bond",
			Name:     b.Name,
			MTU:      maxMTU,
			Bridge:   b.Name,
			VLANTags: getVLANTags(b.Name, b.VLANTags, networkCfg.VLANs),
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("21-_b%s.network", b.Name),
			Contents: buf.String(),
		})

		// Bridge.
		buf.Reset()

		err = bondBridgeT.Execute(&buf, networkFileVariables{
			Type: "bridge",
			Name: b.Name,
			MTU:  maxMTU,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("21-%s.network", b.Name),
			Contents: buf.String(),
		})
	}

	// Create network for each VLAN.
	for _, v := range networkCfg.VLANs {
		configuredMTU := v.MTU
		if configuredMTU == 0 {
			configuredMTU = 1500
		} else if configuredMTU > 9000 {
			configuredMTU = 9000
		}

		buf.Reset()

		err := commonT.Execute(&buf, networkFileVariables{
			Type:              "vlan",
			Name:              v.Name,
			RequiredForOnline: v.RequiredForOnline,
			MTU:               configuredMTU,
			DNS:               networkCfg.DNS,
			TimeConfig:        networkCfg.Time,
			Addresses:         v.Addresses,
			Routes:            v.Routes,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("22-%s.network", v.Name),
			Contents: buf.String(),
		})
	}

	// Create network for each Wireguard.
	for _, wg := range networkCfg.Wireguard {
		configuredMTU := wg.MTU
		if configuredMTU == 0 {
			configuredMTU = 1500
		} else if configuredMTU > 9000 {
			configuredMTU = 9000
		}

		buf.Reset()

		err := commonT.Execute(&buf, networkFileVariables{
			Type:      "wireguard",
			Name:      wg.Name,
			MTU:       configuredMTU,
			Addresses: wg.Addresses,
			Routes:    wg.Routes,
		})
		if err != nil {
			return nil, err
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("23-%s.network", wg.Name),
			Contents: buf.String(),
		})
	}

	return ret, nil
}

func getVLANTags(devName string, additionalVLANTags []int, vlans []api.SystemNetworkVLAN) []int {
	vlanTags := []int{}

	// Add any additional VLAN tags.
	vlanTags = append(vlanTags, additionalVLANTags...)

	// Grab any relevant tags for this device from VLAN definitions.
	for _, vlan := range vlans {
		if vlan.Parent == devName {
			vlanTags = append(vlanTags, vlan.ID)

			break
		}
	}

	// Sort and remove any duplicate tags.
	slices.Sort(vlanTags)

	return slices.Compact(vlanTags)
}

func cleanupStaleDevices(ctx context.Context, oldCfg *api.SystemNetworkConfig, newCfg *api.SystemNetworkConfig) error {
	deleteInterfaces := []string{}

	// Check for changed/deleted interfaces.
	for oldIndex := range oldCfg.Interfaces {
		newIndex := slices.IndexFunc(newCfg.Interfaces, func(i api.SystemNetworkInterface) bool {
			return oldCfg.Interfaces[oldIndex].Name == i.Name
		})

		// If not found, remove the existing interface (either deleted, or the device is now a bond or vlan).
		if newIndex < 0 {
			deleteInterfaces = append(deleteInterfaces, "_v"+oldCfg.Interfaces[oldIndex].Name, oldCfg.Interfaces[oldIndex].Name)

			continue
		}

		// Check if the interface's configuration has changed.
		oldConfig, err := json.Marshal(oldCfg.Interfaces[oldIndex])
		if err != nil {
			return err
		}

		newConfig, err := json.Marshal(newCfg.Interfaces[newIndex])
		if err != nil {
			return err
		}

		if !bytes.Equal(oldConfig, newConfig) {
			deleteInterfaces = append(deleteInterfaces, "_v"+oldCfg.Interfaces[oldIndex].Name)

			if !isBridgeInUse(oldCfg.Interfaces[oldIndex].Name) {
				deleteInterfaces = append(deleteInterfaces, oldCfg.Interfaces[oldIndex].Name)
			}

			continue
		}
	}

	// Check for changed/deleted bonds.
	for oldIndex := range oldCfg.Bonds {
		newIndex := slices.IndexFunc(newCfg.Bonds, func(b api.SystemNetworkBond) bool {
			return oldCfg.Bonds[oldIndex].Name == b.Name
		})

		// If not found, remove the existing bond (either deleted, or the device is now an interface or vlan).
		if newIndex < 0 {
			deleteInterfaces = append(deleteInterfaces, "_b"+oldCfg.Bonds[oldIndex].Name, "_v"+oldCfg.Bonds[oldIndex].Name, oldCfg.Bonds[oldIndex].Name)

			continue
		}

		// Check if the bond's configuration has changed.
		oldConfig, err := json.Marshal(oldCfg.Bonds[oldIndex])
		if err != nil {
			return err
		}

		newConfig, err := json.Marshal(newCfg.Bonds[newIndex])
		if err != nil {
			return err
		}

		if !bytes.Equal(oldConfig, newConfig) {
			deleteInterfaces = append(deleteInterfaces, "_b"+oldCfg.Bonds[oldIndex].Name, "_v"+oldCfg.Bonds[oldIndex].Name)

			if !isBridgeInUse(oldCfg.Bonds[oldIndex].Name) {
				deleteInterfaces = append(deleteInterfaces, oldCfg.Bonds[oldIndex].Name)
			}

			continue
		}
	}

	// Check for changed/deleted vlans.
	for oldIndex := range oldCfg.VLANs {
		newIndex := slices.IndexFunc(newCfg.VLANs, func(v api.SystemNetworkVLAN) bool {
			return oldCfg.VLANs[oldIndex].Name == v.Name
		})

		// If not found, remove the existing vlan (either deleted, or the device is now an interface or bond).
		if newIndex < 0 {
			deleteInterfaces = append(deleteInterfaces, oldCfg.VLANs[oldIndex].Name)

			continue
		}

		// Check if the vlan's configuration has changed.
		oldConfig, err := json.Marshal(oldCfg.VLANs[oldIndex])
		if err != nil {
			return err
		}

		newConfig, err := json.Marshal(newCfg.VLANs[newIndex])
		if err != nil {
			return err
		}

		if !bytes.Equal(oldConfig, newConfig) {
			deleteInterfaces = append(deleteInterfaces, oldCfg.VLANs[oldIndex].Name)

			continue
		}
	}

	// Check for changed/deleted wireguard.
	for oldIndex := range oldCfg.Wireguard {
		newIndex := slices.IndexFunc(newCfg.Wireguard, func(v api.SystemNetworkWireguard) bool {
			return oldCfg.Wireguard[oldIndex].Name == v.Name
		})

		// If not found, remove the existing wireguard.
		if newIndex < 0 {
			deleteInterfaces = append(deleteInterfaces, oldCfg.Wireguard[oldIndex].Name)

			continue
		}

		// Check if the wireguard configuration has changed.
		oldConfig, err := json.Marshal(oldCfg.Wireguard[oldIndex]) // #nosec G117
		if err != nil {
			return err
		}

		newConfig, err := json.Marshal(newCfg.Wireguard[newIndex]) // #nosec G117
		if err != nil {
			return err
		}

		if !bytes.Equal(oldConfig, newConfig) {
			deleteInterfaces = append(deleteInterfaces, oldCfg.Wireguard[oldIndex].Name)

			continue
		}
	}

	// Delete all the interfaces.
	if len(deleteInterfaces) > 0 {
		deleteNetworkDevice(ctx, deleteInterfaces...)
	}

	return nil
}

// isBridgeInUse checks if a bridge exists and only contains internal ports.
func isBridgeInUse(device string) bool {
	entries, err := os.ReadDir(filepath.Join("/sys/class/net", device))
	if err != nil {
		// Not an active interface.
		return false
	}

	isBridge := false
	isUsed := false

	for _, entry := range entries {
		name := entry.Name()

		if name == "bridge" && entry.IsDir() {
			isBridge = true

			continue
		}

		if strings.HasPrefix(name, "lower_") && !strings.HasPrefix(name, "lower__") {
			isUsed = true

			break
		}
	}

	return isBridge && isUsed
}

func deleteNetworkDevice(ctx context.Context, devices ...string) {
	for _, dev := range devices {
		// Ignore errors; when deconstructing a complex network setup we may have already
		// removed one end of a shared device, and we don't want to error out if we fail to
		// remove the other end.
		_, _ = subprocess.RunCommandContext(ctx, "networkctl", "delete", dev)
	}
}

// resolveMACs attempts to resolve any non-MAC looking value to a MAC address by treating the
// value of an interface name and attempting to query its MAC.
func resolveMACs(ctx context.Context, config *api.SystemNetworkConfig) error {
	hwaddrhRegex := regexp.MustCompile(`^[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}$`)

	for i := range len(config.Interfaces) {
		if !hwaddrhRegex.MatchString(config.Interfaces[i].Hwaddr) {
			hwaddr, err := getMacForInterface(ctx, config.Interfaces[i].Hwaddr)
			if err != nil {
				return fmt.Errorf("interface %d failed getting MAC for '%s': %s", i, config.Interfaces[i].Hwaddr, err.Error())
			}

			config.Interfaces[i].Hwaddr = hwaddr
		}
	}

	for i := range len(config.Bonds) {
		if config.Bonds[i].Hwaddr != "" && !hwaddrhRegex.MatchString(config.Bonds[i].Hwaddr) {
			hwaddr, err := getMacForInterface(ctx, config.Bonds[i].Hwaddr)
			if err != nil {
				return fmt.Errorf("bond %d failed getting MAC for '%s': %s", i, config.Bonds[i].Hwaddr, err.Error())
			}

			config.Bonds[i].Hwaddr = hwaddr
		}

		for j := range len(config.Bonds[i].Members) {
			if !hwaddrhRegex.MatchString(config.Bonds[i].Members[j]) {
				hwaddr, err := getMacForInterface(ctx, config.Bonds[i].Members[j])
				if err != nil {
					return fmt.Errorf("bond %d member %d failed getting MAC for '%s': %s", i, j, config.Bonds[i].Members[j], err.Error())
				}

				config.Bonds[i].Members[j] = hwaddr
			}
		}
	}

	return nil
}

// getMacForInterface attempts to query a give network interface and return its MAC address.
func getMacForInterface(ctx context.Context, iface string) (string, error) {
	macAddressRegex := regexp.MustCompile(`link/ether (.+) brd`)

	output, err := subprocess.RunCommandContext(ctx, "ip", "link", "show", iface)
	if err != nil {
		return "", err
	}

	match := macAddressRegex.FindAllStringSubmatch(output, -1)
	if len(match) != 1 {
		return "", errors.New("no MAC address found")
	}

	return match[0][1], nil
}

// Determine the maximum MTU supported by a given device. Because device names can change,
// we use the underlying MAC when getting the maximum MTU.
func getMaxMTUForMAC(ctx context.Context, mac string) (int, error) {
	// To support testing, if an empty context is provided, return a
	// default unique maximum MTU.
	if ctx == context.TODO() {
		return 9009, nil
	}

	mtuRegex := regexp.MustCompile(strings.ToLower(mac) + ` .+? minmtu \d+ maxmtu (\d+)`)

	output, err := subprocess.RunCommandContext(ctx, "ip", "-d", "link")
	if err != nil {
		return -1, err
	}

	match := mtuRegex.FindAllStringSubmatch(output, -1)
	if len(match) == 0 {
		slog.WarnContext(ctx, "unable to determine maximum MTU for "+mac+", defaulting to 1500")

		return 1500, nil
	}

	mtu, err := strconv.Atoi(match[0][1])
	if err != nil {
		return -1, err
	}

	// Cap maximum MTU to 9000.
	if mtu > 9000 {
		mtu = 9000
	}

	// Make sure the MTU isn't too small for IPv6.
	if mtu < 1280 {
		return -1, fmt.Errorf("maximum MTU of %d for %s is too small to support IPv6", mtu, mac)
	}

	return mtu, nil
}

func getExpectedNewPhysicalDevices(ctx context.Context, config *api.SystemNetworkConfig) []expPhysDev {
	devices := []expPhysDev{}
	ret := []expPhysDev{}

	// Get a list of all the expected "_p" physical devices referenced by the interfaces or bond
	// members in the given network configuration.
	for i := range config.Interfaces {
		devices = append(devices, expPhysDev{
			Name:      config.Interfaces[i].Name,
			Interface: "_p" + strings.ToLower(strings.ReplaceAll(config.Interfaces[i].Hwaddr, ":", "")),
			Hwaddr:    strings.ToLower(config.Interfaces[i].Hwaddr),
		})
	}

	for i := range config.Bonds {
		for j := range config.Bonds[i].Members {
			devices = append(devices, expPhysDev{
				Name:      config.Bonds[i].Name,
				Interface: "_p" + strings.ToLower(strings.ReplaceAll(config.Bonds[i].Members[j], ":", "")),
				Hwaddr:    strings.ToLower(config.Bonds[i].Members[j]),
			})
		}
	}

	// Check if the given device is already known to networkd; if not, add it to the list
	// of devices we need to wait for.
	for _, dev := range devices {
		_, err := subprocess.RunCommandContext(ctx, "networkctl", "status", dev.Interface)
		if err != nil {
			ret = append(ret, dev)
		}
	}

	return ret
}

func mangleUSBNICs(config *api.SystemNetworkConfig) {
	usbNICRegex := regexp.MustCompile(`^enx[[:xdigit:]]{12}$`)

	for i := range config.Interfaces {
		if usbNICRegex.MatchString(config.Interfaces[i].Name) {
			config.Interfaces[i].Name = strings.TrimPrefix(config.Interfaces[i].Name, "enx")
		}
	}
}

func checkWireguardPrivateKeys(ctx context.Context, networkCfg *api.SystemNetworkConfig) error {
	// Check for a private key for each wireguard.
	for index, wg := range networkCfg.Wireguard {
		// No private key defined generate one as this is required for wireguard.
		if wg.PrivateKey == "" {
			output, err := subprocess.RunCommandContext(ctx, "wg", "genkey")
			if err != nil {
				return err
			}

			networkCfg.Wireguard[index].PrivateKey = strings.Trim(output, "\n")
		}
	}

	return nil
}
