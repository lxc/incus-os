package seed

import (
	"context"
	"net"
)

// GetNetwork extracts the network configuration from the seed data.
// If no seed network found, a default minimal network config will be returned.
func GetNetwork(_ context.Context, partition string) (*NetworkConfig, error) {
	// Get the network configuration.
	var config NetworkConfig

	err := parseFileContents(partition, "network", &config)
	if err != nil {
		// No seed network available; return a minimal default.
		defaultNetwork, err := getDefaultNetworkConfig()
		if err != nil {
			return nil, err
		}

		return defaultNetwork, nil
	}

	// If no interfaces, bonds, or vlans are defined, add a minimal default configuration for the interfaces.
	if networkConfigHasEmptyDevices(config) {
		defaultNetwork, err := getDefaultNetworkConfig()
		if err != nil {
			return nil, err
		}
		config.Interfaces = defaultNetwork.Interfaces
	}

	return &config, nil
}

// networkConfigHasEmptyDevices checks if any device (interface, bond, or vlan) is defined in the given config.
func networkConfigHasEmptyDevices(networkCfg NetworkConfig) bool {
	return len(networkCfg.Interfaces) == 0 && len(networkCfg.Bonds) == 0 && len(networkCfg.Vlans) == 0
}

// getDefaultNetworkConfig returns a minimal network configuration, with every interface
// configured to acquire an IP via DHCP and SLAAC.
func getDefaultNetworkConfig() (*NetworkConfig, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ret := new(NetworkConfig)

	for _, i := range interfaces {
		if i.Name == "lo" {
			continue
		}

		ret.Interfaces = append(ret.Interfaces, NetworkInterface{
			Name:      i.Name,
			Hwaddr:    i.HardwareAddr.String(),
			Addresses: []string{"dhcp4", "slaac"},
		})
	}

	return ret, nil
}
