package seed

import (
	"context"
	"net"

	"github.com/lxc/incus-os/incus-osd/api"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

// GetNetwork extracts the network configuration from the seed data.
// If no seed network found, a default minimal network config will be returned.
func GetNetwork(_ context.Context, partition string) (*api.SystemNetworkConfig, error) {
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

	// If no interfaces, bonds, or vlans are defined, add a minimal default configuration for the interfaces.
	if NetworkConfigHasEmptyDevices(config.SystemNetworkConfig) {
		defaultNetwork, err := getDefaultNetworkConfig()
		if err != nil {
			return nil, err
		}

		config.Interfaces = defaultNetwork.Interfaces
	}

	// If no timezone was provided, default to UTC.
	if config.Time == nil {
		config.Time = &api.SystemNetworkTime{}
	}

	if config.Time.Timezone == "" {
		config.Time.Timezone = "UTC"
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

	ret := &api.SystemNetworkConfig{
		Time: &api.SystemNetworkTime{
			Timezone: "UTC",
		},
	}

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
