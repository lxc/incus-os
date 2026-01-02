package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/lxc/incus/v6/shared/subprocess"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// ApplyKernelConfiguration updates various parts of the kernel configuration. A reboot
// may be required to fully apply the changes.
func ApplyKernelConfiguration(ctx context.Context, config api.SystemKernelConfig) error {
	// Update the list of blacklisted kernel modules.
	err := updateBlacklistModules(config.BlacklistModules)
	if err != nil {
		return err
	}

	// Update network sysctl configuration.
	err = updateNetworkSysctlConfig(ctx, config.Network)
	if err != nil {
		return err
	}

	// Update the list of PCI(e) pass-throughs.
	if config.PCI != nil {
		err := updatePCIPassthroughs(config.PCI.Passthrough)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateBlacklistModules(modules []string) error {
	// Remove the existing configuration file, if it exists.
	err := os.Remove("/etc/modprobe.d/99-blacklist-modules.conf")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If no blacklisted modules are specified, there's nothing else to do.
	if len(modules) == 0 {
		return nil
	}

	// Ensure the modprobe.d directory exists.
	err = os.MkdirAll("/etc/modprobe.d/", 0o755)
	if err != nil {
		return err
	}

	// Create the new configuration file.
	fd, err := os.Create("/etc/modprobe.d/99-blacklist-modules.conf")
	if err != nil {
		return err
	}
	defer fd.Close()

	// Write the blacklist configuration and opportunistically unload each module.
	for _, module := range modules {
		if module == "" {
			continue
		}

		_, err := fd.WriteString("blacklist " + module + "\n")
		if err != nil {
			return err
		}

		// Ignore any errors encountered attempting to unload the module.
		_, _ = subprocess.RunCommand("/sbin/rmmod", module)
	}

	return nil
}

func updateNetworkSysctlConfig(ctx context.Context, config *api.SystemKernelConfigNetwork) error {
	// Perform simple validation checks.
	if config != nil {
		if config.BufferSize < 0 {
			return errors.New("buffer size cannot be negative")
		}

		if config.TCPCongestionAlgorithm != "" {
			output, err := subprocess.RunCommand("sysctl", "-n", "net.ipv4.tcp_available_congestion_control")
			if err != nil {
				return err
			}

			output = strings.TrimSuffix(output, "\n")

			if !slices.Contains(strings.Split(output, " "), config.TCPCongestionAlgorithm) {
				return fmt.Errorf("unsupported tcp_congestion_control value, must be one of %v", strings.Split(output, " "))
			}
		}
	}

	// Remove the existing configuration file, if it exists.
	err := os.Remove("/etc/sysctl.d/99-local-network-sysctl.conf")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If no sysctls are provided, there's nothing else to do.
	if config == nil {
		// Restart the systemd-sysctl service to pickup changes.
		return systemd.RestartUnit(ctx, "systemd-sysctl.service")
	}

	// Ensure the sysctl.d directory exists.
	err = os.MkdirAll("/etc/sysctl.d/", 0o755)
	if err != nil {
		return err
	}

	// Create the new configuration file.
	fd, err := os.Create("/etc/sysctl.d/99-local-network-sysctl.conf")
	if err != nil {
		return err
	}
	defer fd.Close()

	// Write the file contents.
	if config.BufferSize != 0 {
		_, err := fmt.Fprintf(fd, "net.ipv4.tcp_rmem = 4096 65536 %d\nnet.ipv4.tcp_wmem = 4096 65536 %d\nnet.core.rmem_max = %d\nnet.core.wmem_max = %d\n", config.BufferSize, config.BufferSize, config.BufferSize, config.BufferSize)
		if err != nil {
			return err
		}
	}

	if config.QueuingDiscipline != "" {
		_, err := fd.WriteString("net.core.default_qdisc = " + config.QueuingDiscipline + "\n")
		if err != nil {
			return err
		}
	}

	if config.TCPCongestionAlgorithm != "" {
		_, err := fd.WriteString("net.ipv4.tcp_congestion_control = " + config.TCPCongestionAlgorithm + "\n")
		if err != nil {
			return err
		}
	}

	// Restart the systemd-sysctl service to pickup changes.
	return systemd.RestartUnit(ctx, "systemd-sysctl.service")
}

func updatePCIPassthroughs(config []api.SystemKernelConfigPCIPassthrough) error {
	// Verify that Vendor and Device IDs look plausible.
	isHex := regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	for _, c := range config {
		if !isHex.MatchString(c.VendorID) {
			return errors.New("Vendor ID '" + c.VendorID + "' is invalid")
		}

		if !isHex.MatchString(c.ProductID) {
			return errors.New("Product ID '" + c.ProductID + "' is invalid")
		}
	}

	// Remove the existing configuration file, if it exists.
	err := os.Remove("/etc/modprobe.d/99-local-device-passthrough.conf")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If no passthroughs are specified, there's nothing else to do.
	if len(config) == 0 {
		return nil
	}

	// Ensure the modprobe.d directory exists.
	err = os.MkdirAll("/etc/modprobe.d/", 0o755)
	if err != nil {
		return err
	}

	// Create the new configuration file.
	fd, err := os.Create("/etc/modprobe.d/99-local-device-passthrough.conf")
	if err != nil {
		return err
	}
	defer fd.Close()

	pciIDs := []string{}
	for _, c := range config {
		pciIDs = append(pciIDs, c.VendorID+":"+c.ProductID)

		// If a specific PCI address is provided, attempt to opportunistically unbind/bind that specific device.
		if c.PCIAddress != "" {
			// Unbind.
			oldFd, err := os.Open("/sys/bus/pci/devices/" + c.PCIAddress + "/driver/unbind")
			if err != nil {
				continue
			}
			defer oldFd.Close() //nolint:revive

			_, err = oldFd.WriteString(c.PCIAddress + "\n")
			if err != nil {
				continue
			}

			// Attempt to bind all VendorID:ProductID devices to vfio-pci.
			newFd, err := os.Open("/sys/bus/pci/drivers/vfio-pci/new_id")
			if err != nil {
				continue
			}
			defer newFd.Close() //nolint:revive

			_, err = newFd.WriteString(c.VendorID + " " + c.ProductID + "\n")
			if err != nil {
				continue
			}
		}
	}

	// Write the file contents.
	_, err = fd.WriteString("options vfio-pci ids=" + strings.Join(pciIDs, ",") + "\n")

	return err
}
