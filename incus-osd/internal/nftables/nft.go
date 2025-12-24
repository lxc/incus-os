package nftables

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
)

// SetupChains creates the initial system-wide chains.
func SetupChains(ctx context.Context) error {
	// Ensure we have an inet table.
	_, err := subprocess.RunCommandContext(ctx, "nft", "add", "table", "inet", "incus-osd")
	if err != nil {
		return err
	}

	// Ensure we have an input filtering chain.
	_, err = subprocess.RunCommandContext(ctx, "nft", "add", "chain", "inet", "incus-osd", "input", "{ type filter hook input priority 0 ; policy accept ; }")
	if err != nil {
		return err
	}

	// Ensure we have a bridge table.
	_, err = subprocess.RunCommandContext(ctx, "nft", "add", "table", "bridge", "incus-osd")
	if err != nil {
		return err
	}

	// Ensure we have a MAC filtering chain.
	_, err = subprocess.RunCommandContext(ctx, "nft", "add", "chain", "bridge", "incus-osd", "mac-filters", "{ type filter hook output priority 0 ; policy accept ; }")
	if err != nil {
		return err
	}

	return nil
}

// ApplyHwaddrFilters ensures that all interfaces with the StrictHwaddr flag set get a suitable MAC filter in place.
func ApplyHwaddrFilters(ctx context.Context, networkCfg *api.SystemNetworkConfig) error {
	// Make sure we have the expected chains.
	err := SetupChains(ctx)
	if err != nil {
		return err
	}

	// Empty the chain.
	_, err = subprocess.RunCommandContext(ctx, "nft", "flush", "chain", "bridge", "incus-osd", "mac-filters")
	if err != nil {
		return err
	}

	// Apply the filters.
	for _, iface := range networkCfg.Interfaces {
		if !iface.StrictHwaddr {
			continue
		}

		underlyingDevice := "_p" + strings.ToLower(strings.ReplaceAll(iface.Hwaddr, ":", ""))

		_, err = subprocess.RunCommandContext(ctx, "nft", "add", "rule", "bridge", "incus-osd", "mac-filters", "oifname", underlyingDevice, "ether", "saddr", "!=", iface.Hwaddr, "drop")
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyInputFilters applies the input firewall rules.
func ApplyInputFilters(ctx context.Context, networkCfg *api.SystemNetworkConfig) error {
	// Make sure we have the expected chains.
	err := SetupChains(ctx)
	if err != nil {
		return err
	}

	// Empty the chain.
	_, err = subprocess.RunCommandContext(ctx, "nft", "flush", "chain", "inet", "incus-osd", "input")
	if err != nil {
		return err
	}

	// Apply the filters.
	applyFirewall := func(iface string, firewallRules []api.SystemNetworkFirewallRule) error {
		// Baseline rules.
		rules := [][]string{
			{"ct", "state", "established,related", "accept"},
			{"ct", "state", "invalid", "drop"},
			{"ip", "protocol", "icmp", "accept"},
			{"icmp", "type", "{echo-request,destination-unreachable,time-exceeded,parameter-problem}", "accept"},
			{"icmpv6", "type", "{echo-request,nd-neighbor-solicit,nd-neighbor-advert,nd-router-solicit,nd-router-advert,mld-listener-query}", "accept"},
		}

		// Add the user rules.
		for _, firewallRule := range firewallRules {
			rule := []string{}

			if firewallRule.Source != "" {
				var ip net.IP

				if strings.Contains(firewallRule.Source, "/") {
					var err error

					ip, _, err = net.ParseCIDR(firewallRule.Source)
					if err != nil {
						return err
					}
				} else {
					ip = net.ParseIP(firewallRule.Source)
				}

				if ip == nil {
					return fmt.Errorf("bad source %q", firewallRule.Source)
				}

				if ip.To4() == nil {
					rule = append(rule, "ip6", "saddr", firewallRule.Source)
				} else {
					rule = append(rule, "ip", "saddr", firewallRule.Source)
				}
			}

			if firewallRule.Protocol != "" {
				rule = append(rule, firewallRule.Protocol, "dport", strconv.Itoa(firewallRule.Port))
			}

			rule = append(rule, firewallRule.Action)
			rules = append(rules, rule)
		}

		// Apply the interface rules.
		args := []string{"add", "rule", "inet", "incus-osd", "input", "iifname", iface}
		for _, rule := range rules {
			_, err = subprocess.RunCommandContext(ctx, "nft", append(args, rule...)...)
			if err != nil {
				return err
			}
		}

		return nil
	}

	for _, iface := range networkCfg.Interfaces {
		if len(iface.FirewallRules) == 0 {
			continue
		}

		err := applyFirewall("_v"+iface.Name, iface.FirewallRules)
		if err != nil {
			return err
		}
	}

	for _, iface := range networkCfg.Bonds {
		if len(iface.FirewallRules) == 0 {
			continue
		}

		err := applyFirewall(iface.Name, iface.FirewallRules)
		if err != nil {
			return err
		}
	}

	for _, iface := range networkCfg.VLANs {
		if len(iface.FirewallRules) == 0 {
			continue
		}

		err := applyFirewall(iface.Name, iface.FirewallRules)
		if err != nil {
			return err
		}
	}

	for _, iface := range networkCfg.Wireguard {
		if len(iface.FirewallRules) == 0 {
			continue
		}

		err := applyFirewall(iface.Name, iface.FirewallRules)
		if err != nil {
			return err
		}
	}

	return nil
}
