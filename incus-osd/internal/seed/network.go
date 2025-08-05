package seed

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetNetwork extracts the network configuration from the seed data.
// If no seed network found, a default minimal network config will be returned.
func GetNetwork(ctx context.Context, partition string) (*api.SystemNetworkConfig, error) {
	// Get the network configuration.
	var config apiseed.Network

	err := parseFileContents(partition, "network", &config)
	if err != nil {
		if !IsMissing(err) {
			return nil, err
		}

		// No seed network available; return a minimal default.
		defaultNetwork, err := getDefaultNetworkConfig()
		if err != nil {
			return nil, err
		}

		return defaultNetwork, nil
	}

	// When reading in a network seed, we will dynamically lookup any MAC address that is referred to by an interface name.
	hwaddrhRegex := regexp.MustCompile(`^[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}$`)

	for i := range len(config.Interfaces) {
		if !hwaddrhRegex.MatchString(config.Interfaces[i].Hwaddr) {
			hwaddr, err := getMacForInterface(ctx, config.Interfaces[i].Hwaddr)
			if err != nil {
				return nil, fmt.Errorf("interface %d failed getting MAC for '%s': %s", i, config.Interfaces[i].Hwaddr, err.Error())
			}

			config.Interfaces[i].Hwaddr = hwaddr
		}
	}

	for i := range len(config.Bonds) {
		if config.Bonds[i].Hwaddr != "" && !hwaddrhRegex.MatchString(config.Bonds[i].Hwaddr) {
			hwaddr, err := getMacForInterface(ctx, config.Bonds[i].Hwaddr)
			if err != nil {
				return nil, fmt.Errorf("bond %d failed getting MAC for '%s': %s", i, config.Bonds[i].Hwaddr, err.Error())
			}

			config.Bonds[i].Hwaddr = hwaddr
		}

		for j := range len(config.Bonds[i].Members) {
			if !hwaddrhRegex.MatchString(config.Bonds[i].Members[j]) {
				hwaddr, err := getMacForInterface(ctx, config.Bonds[i].Members[j])
				if err != nil {
					return nil, fmt.Errorf("bond %d member %d failed getting MAC for '%s': %s", i, j, config.Bonds[i].Members[j], err.Error())
				}

				config.Bonds[i].Members[j] = hwaddr
			}
		}
	}

	// If no interfaces, bonds, or vlans are defined, add a minimal default configuration for the interfaces.
	if NetworkConfigHasEmptyDevices(config.SystemNetworkConfig) {
		defaultNetwork, err := getDefaultNetworkConfig()
		if err != nil {
			return nil, err
		}

		config.Interfaces = defaultNetwork.Interfaces
	}

	return &config.SystemNetworkConfig, nil
}

// NetworkConfigHasEmptyDevices checks if any device (interface, bond, or vlan) is defined in the given config.
func NetworkConfigHasEmptyDevices(networkCfg api.SystemNetworkConfig) bool {
	return len(networkCfg.Interfaces) == 0 && len(networkCfg.Bonds) == 0 && len(networkCfg.VLANs) == 0
}

// getDefaultNetworkConfig returns a minimal network configuration, with every interface
// configured to acquire an IP via DHCP and SLAAC.
func getDefaultNetworkConfig() (*api.SystemNetworkConfig, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ret := &api.SystemNetworkConfig{}

	for _, i := range interfaces {
		if i.Name == "lo" {
			continue
		}

		// Auto-generated interfaces set RequiredForOnline to "no", and we rely
		// on the DNS check after interface configuration to indicate when there's
		// good network connectivity.
		ret.Interfaces = append(ret.Interfaces, api.SystemNetworkInterface{
			Name:              i.Name,
			Hwaddr:            i.HardwareAddr.String(),
			Addresses:         []string{"dhcp4", "slaac"},
			RequiredForOnline: "no",
		})
	}

	return ret, nil
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
