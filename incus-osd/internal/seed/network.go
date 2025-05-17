package seed

import (
	"context"
	"net"

	"github.com/lxc/incus-os/incus-osd/api"
)

// NetworkSeed defines a struct to hold network configuration.
type NetworkSeed struct {
	api.SystemNetworkConfig

	Version string `json:"version" yaml:"version"`
}

// GetNetwork extracts the network configuration from the seed data.
// If no seed network found, a default minimal network config will be returned.
func GetNetwork(_ context.Context, partition string) (*api.SystemNetworkConfig, error) {
	// Get the network configuration.
	var config NetworkSeed

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

		ret.Interfaces = append(ret.Interfaces, api.SystemNetworkInterface{
			Name:      i.Name,
			Hwaddr:    i.HardwareAddr.String(),
			Addresses: []string{"dhcp4", "slaac"},
		})
	}

	return ret, nil
}
