package systemd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/incus-os/incus-osd/internal/seed"
)

// networkdConfigFile represents a given filename and its contents.
type networkdConfigFile struct {
	Name     string
	Contents string
}

// generateNetworkConfiguration clears any existing configuration from /run/systemd/network/ and generates
// new config files from the supplied NetworkConfig struct.
func generateNetworkConfiguration(_ context.Context, networkCfg *seed.NetworkConfig) error {
	// Remove any existing configuration.
	err := os.RemoveAll(SystemdNetworkConfigPath)
	if err != nil {
		return err
	}

	err = os.Mkdir(SystemdNetworkConfigPath, 0o755) //nolint:gosec
	if err != nil {
		return err
	}

	// When no configuration is provided, just DHCP on everything..
	if networkCfg == nil {
		return os.WriteFile(filepath.Join(SystemdNetworkConfigPath, "00-default.network"), []byte( //nolint:gosec
			`[Match]
Name=en*

[Network]
DHCP=ipv4
LinkLocalAddressing=ipv6

[DHCP]
ClientIdentifier=mac
RouteMetric=100
UseMTU=true`), 0o644)
	}

	// Generate .link files.
	for _, cfg := range generateLinkFileContents(*networkCfg) {
		err := os.WriteFile(filepath.Join(SystemdNetworkConfigPath, cfg.Name), []byte(cfg.Contents), 0o644) //nolint:gosec
		if err != nil {
			return err
		}
	}

	// Generate .netdev files.
	for _, cfg := range generateNetdevFileContents(*networkCfg) {
		err := os.WriteFile(filepath.Join(SystemdNetworkConfigPath, cfg.Name), []byte(cfg.Contents), 0o644) //nolint:gosec
		if err != nil {
			return err
		}
	}

	// Generate .network files.
	for _, cfg := range generateNetworkFileContents(*networkCfg) {
		err := os.WriteFile(filepath.Join(SystemdNetworkConfigPath, cfg.Name), []byte(cfg.Contents), 0o644) //nolint:gosec
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyNetworkConfiguration instructs systemd-networkd to apply the supplied network configuration.
func ApplyNetworkConfiguration(ctx context.Context, networkCfg *seed.NetworkConfig) error {
	err := generateNetworkConfiguration(ctx, networkCfg)
	if err != nil {
		return err
	}

	// Trigger udev rule update to pickup device names.
	err = RestartUnit(ctx, "systemd-udev-trigger")
	if err != nil {
		return err
	}

	// Restart networking after new config files have been generated.
	return RestartUnit(ctx, "systemd-networkd")
}

// generateLinkFileContents generates the contents of systemd.link files. Returns an array of ConfigFile structs.
// https://www.freedesktop.org/software/systemd/man/latest/systemd.link.html
func generateLinkFileContents(networkCfg seed.NetworkConfig) []networkdConfigFile {
	ret := []networkdConfigFile{}

	for _, i := range networkCfg.Interfaces {
		strippedHwaddr := strings.ToLower(strings.ReplaceAll(i.Hwaddr, ":", ""))
		ret = append(ret, networkdConfigFile{
			Name: fmt.Sprintf("00-en%s.link", strippedHwaddr),
			Contents: fmt.Sprintf(`[Match]
MACAddress=%s

[Link]
NamePolicy=
Name=en%s
`, i.Hwaddr, strippedHwaddr),
		})
	}

	return ret
}

// generateNetdevFileContents generates the contents of systemd.netdev files. Returns an array of networkdConfigFile structs.
// https://www.freedesktop.org/software/systemd/man/latest/systemd.netdev.html
func generateNetdevFileContents(networkCfg seed.NetworkConfig) []networkdConfigFile {
	ret := []networkdConfigFile{}

	// Create bridge devices for each interface.
	for _, i := range networkCfg.Interfaces {
		ret = append(ret, networkdConfigFile{
			Name: fmt.Sprintf("00-%s.netdev", i.Name),
			Contents: fmt.Sprintf(`[NetDev]
Name=%s
Kind=bridge

[Bridge]
VLANFiltering=true
`, i.Name),
		})
	}

	// Create bonds.
	for _, b := range networkCfg.Bonds {
		ret = append(ret, networkdConfigFile{
			Name: fmt.Sprintf("00-bn%s.netdev", b.Name),
			Contents: fmt.Sprintf(`[NetDev]
Name=bn%s
Kind=bond
MTUBytes=%d

[Bond]
Mode=%s
`, b.Name, b.MTU, b.Mode),
		})
	}

	// Create vlans.
	for _, v := range networkCfg.Vlans {
		ret = append(ret, networkdConfigFile{
			Name: fmt.Sprintf("00-vl%s.netdev", v.Name),
			Contents: fmt.Sprintf(`[NetDev]
Name=vl%s
Kind=vlan
MTUBytes=%d

[Bridge]
Id=%d
`, v.Name, v.MTU, v.ID),
		})
	}

	return ret
}

// generateNetworkFileContents generates the contents of systemd.network files. Returns an array of networkdConfigFile structs.
// https://www.freedesktop.org/software/systemd/man/latest/systemd.network.html
func generateNetworkFileContents(networkCfg seed.NetworkConfig) []networkdConfigFile {
	ret := []networkdConfigFile{}

	// Create networks for each interface.
	for _, i := range networkCfg.Interfaces {
		strippedHwaddr := strings.ToLower(strings.ReplaceAll(i.Hwaddr, ":", ""))
		cfgString := fmt.Sprintf(`[Match]
Name=%s

[Network]
`, i.Name)

		cfgString += processAddresses(i.Addresses)

		if len(i.Routes) > 0 {
			cfgString += "\n[Route]\n"
			cfgString += processRoutes(i.Routes)
		}

		if i.VLAN != 0 {
			cfgString += fmt.Sprintf("\n[BridgeVLAN]\nVLAN=%d\n", i.VLAN)
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("00-%s.network", i.Name),
			Contents: cfgString,
		})

		ret = append(ret, networkdConfigFile{
			Name: fmt.Sprintf("00-en%s.network", strippedHwaddr),
			Contents: fmt.Sprintf(`[Match]
Name=en%s

[Network]
Bridge=%s
`, strippedHwaddr, i.Name),
		})
	}

	// Create networks for each bond.
	for _, b := range networkCfg.Bonds {
		cfgString := fmt.Sprintf(`[Match]
Name=bn%s

[Network]
`, b.Name)

		cfgString += processAddresses(b.Addresses)

		if len(b.Routes) > 0 {
			cfgString += "\n[Route]\n"
			cfgString += processRoutes(b.Routes)
		}

		if b.VLAN != 0 {
			cfgString += fmt.Sprintf("\n[BridgeVLAN]\nVLAN=%d\n", b.VLAN)
		}

		ret = append(ret, networkdConfigFile{
			Name:     fmt.Sprintf("00-bn%s.network", b.Name),
			Contents: cfgString,
		})

		for index, member := range b.Members {
			ret = append(ret, networkdConfigFile{
				Name: fmt.Sprintf("00-bn%s-dev%d.network", b.Name, index),
				Contents: fmt.Sprintf(`[Match]
MACAddress=%s

[Network]
Bond=bn%s
`, member, b.Name),
			})
		}
	}

	return ret
}

func processAddresses(addresses []string) string {
	ret := ""

	hasDHCP4 := false
	hasDHCP6 := false
	for _, addr := range addresses {
		switch addr {
		case "dhcp4":
			hasDHCP4 = true
		case "dhcp6":
			hasDHCP6 = true
			ret += "IPv6AcceptRA=false\n"
		case "slaac":
			ret += "IPv6AcceptRA=true\n"
		default:
			ret += fmt.Sprintf("Address=%s\n", addr)
		}
	}

	if hasDHCP4 && hasDHCP6 { //nolint:gocritic
		ret += "DHCP=yes\n"
	} else if hasDHCP4 {
		ret += "DHCP=ipv4\n"
	} else if hasDHCP6 {
		ret += "DHCP=ipv6\n"
	}

	return ret
}

func processRoutes(routes []seed.Route) string {
	ret := ""

	for _, route := range routes {
		switch route.Via {
		case "dhcp4":
			ret += "Gateway=_dhcp4\n"
		case "slaac":
			ret += "Gateway=_ipv6ra\n"
		default:
			ret += fmt.Sprintf("Gateway=%s\n", route.Via)
		}

		ret += fmt.Sprintf("Destination=%s\n", route.To)
	}

	return ret
}
