package systemd

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/lxc/incus-os/incus-osd/api"
)

func validateInterfaces(interfaces []api.SystemNetworkInterface, vlans []api.SystemNetworkVLAN, requireValidMAC bool) error {
	for index, iface := range interfaces {
		err := validateName(iface.Name)
		if err != nil {
			return fmt.Errorf("interface %d %s", index, err.Error())
		}

		err = validateMTU(iface.MTU)
		if err != nil {
			return fmt.Errorf("interface %d %s", index, err.Error())
		}

		err = validateRoles(iface.Roles)
		if err != nil {
			return fmt.Errorf("interface %d %s", index, err.Error())
		}

		if iface.VLAN != 0 {
			err := validateVLAN(iface.VLAN, vlans)
			if err != nil {
				return fmt.Errorf("interface %d %s", index, err.Error())
			}
		}

		for addressIndex, address := range iface.Addresses {
			err := validateAddress(address)
			if err != nil {
				return fmt.Errorf("interface %d address %d %s", index, addressIndex, err.Error())
			}
		}

		err = validateRequiredForOnline(iface.RequiredForOnline)
		if err != nil {
			return fmt.Errorf("interface %d %s", index, err.Error())
		}

		for routeIndex, route := range iface.Routes {
			err := validateAddress(route.To)
			if err != nil {
				return fmt.Errorf("interface %d route %d 'To' %s", index, routeIndex, err.Error())
			}

			err = validateAddress(route.Via)
			if err != nil {
				return fmt.Errorf("interface %d route %d 'Via' %s", index, routeIndex, err.Error())
			}
		}

		err = validateHwaddr(iface.Hwaddr, requireValidMAC)
		if err != nil {
			return fmt.Errorf("interface %d %s", index, err.Error())
		}
	}

	return nil
}

func validateBonds(bonds []api.SystemNetworkBond, vlans []api.SystemNetworkVLAN, requireValidMAC bool) error {
	for index, bond := range bonds {
		err := validateName(bond.Name)
		if err != nil {
			return fmt.Errorf("bond %d %s", index, err.Error())
		}

		err = validateMode(bond.Mode)
		if err != nil {
			return fmt.Errorf("bond %d %s", index, err.Error())
		}

		err = validateMTU(bond.MTU)
		if err != nil {
			return fmt.Errorf("bond %d %s", index, err.Error())
		}

		err = validateRoles(bond.Roles)
		if err != nil {
			return fmt.Errorf("bond %d %s", index, err.Error())
		}

		if bond.VLAN != 0 {
			err := validateVLAN(bond.VLAN, vlans)
			if err != nil {
				return fmt.Errorf("bond %d %s", index, err.Error())
			}
		}

		for addressIndex, address := range bond.Addresses {
			err := validateAddress(address)
			if err != nil {
				return fmt.Errorf("bond %d address %d %s", index, addressIndex, err.Error())
			}
		}

		err = validateRequiredForOnline(bond.RequiredForOnline)
		if err != nil {
			return fmt.Errorf("bond %d %s", index, err.Error())
		}

		for routeIndex, route := range bond.Routes {
			err := validateAddress(route.To)
			if err != nil {
				return fmt.Errorf("bond %d route %d 'To' %s", index, routeIndex, err.Error())
			}

			err = validateAddress(route.Via)
			if err != nil {
				return fmt.Errorf("bond %d route %d 'Via' %s", index, routeIndex, err.Error())
			}
		}

		if bond.Hwaddr != "" {
			err = validateHwaddr(bond.Hwaddr, requireValidMAC)
			if err != nil {
				return fmt.Errorf("bond %d %s", index, err.Error())
			}
		}

		if len(bond.Members) == 0 {
			return fmt.Errorf("bond %d has no members", index)
		}

		for memberIndex, member := range bond.Members {
			err := validateHwaddr(member, requireValidMAC)
			if err != nil {
				return fmt.Errorf("bond %d member %d %s", index, memberIndex, err.Error())
			}
		}
	}

	return nil
}

func validateVLANs(cfg *api.SystemNetworkConfig) error {
	for index, vlan := range cfg.VLANs {
		err := validateName(vlan.Name)
		if err != nil {
			return fmt.Errorf("vlan %d %s", index, err.Error())
		}

		err = validateParent(vlan.Parent, cfg.Interfaces, cfg.Bonds)
		if err != nil {
			return fmt.Errorf("vlan %d %s", index, err.Error())
		}

		if vlan.ID < 0 || vlan.ID > 4094 {
			return fmt.Errorf("vlan %d ID %d out of range", index, vlan.ID)
		}

		err = validateMTU(vlan.MTU)
		if err != nil {
			return fmt.Errorf("vlan %d %s", index, err.Error())
		}

		err = validateRoles(vlan.Roles)
		if err != nil {
			return fmt.Errorf("vlan %d %s", index, err.Error())
		}

		for addressIndex, address := range vlan.Addresses {
			err := validateAddress(address)
			if err != nil {
				return fmt.Errorf("vlan %d address %d %s", index, addressIndex, err.Error())
			}
		}

		err = validateRequiredForOnline(vlan.RequiredForOnline)
		if err != nil {
			return fmt.Errorf("vlan %d %s", index, err.Error())
		}

		for routeIndex, route := range vlan.Routes {
			err := validateAddress(route.To)
			if err != nil {
				return fmt.Errorf("vlan %d route %d 'To' %s", index, routeIndex, err.Error())
			}

			err = validateAddress(route.Via)
			if err != nil {
				return fmt.Errorf("vlan %d route %d 'Via' %s", index, routeIndex, err.Error())
			}
		}
	}

	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("has no name")
	}

	if strings.HasPrefix(name, "_") {
		return errors.New("name cannot begin with an underscore")
	}

	if len(name) > 13 {
		return errors.New("name cannot be longer than 13 characters")
	}

	return nil
}

func validateMode(mode string) error {
	if mode != "balance-rr" && mode != "active-backup" && mode != "balance-xor" && mode != "broadcast" && mode != "802.3ad" && mode != "balance-tlb" && mode != "balance-alb" {
		return fmt.Errorf("invalid Mode value '%s'", mode)
	}

	return nil
}

func validateParent(parent string, interfaces []api.SystemNetworkInterface, bonds []api.SystemNetworkBond) error {
	if parent == "" {
		return errors.New("has no parent")
	}

	foundParent := false

	for _, i := range interfaces {
		if i.Name == parent {
			foundParent = true

			break
		}
	}

	if !foundParent {
		for _, b := range bonds {
			if b.Name == parent {
				foundParent = true

				break
			}
		}
	}

	if !foundParent {
		return fmt.Errorf("unable to find parent '%s'", parent)
	}

	return nil
}

func validateRoles(roles []string) error {
	existing := make([]string, 0, len(roles))

	for _, role := range roles {
		// Confirm role is valid.
		if !slices.Contains([]string{api.SystemNetworkInterfaceRoleManagement, api.SystemNetworkInterfaceRoleCluster, api.SystemNetworkInterfaceRoleInstances, api.SystemNetworkInterfaceRoleStorage}, role) {
			return fmt.Errorf("role %q is unsupported", role)
		}

		// Duplicate detection.
		if slices.Contains(existing, role) {
			return fmt.Errorf("role %q is listed multiple times", role)
		}

		existing = append(existing, role)
	}

	return nil
}

func validateMTU(mtu int) error {
	if mtu < 0 || mtu > 9000 {
		return errors.New("MTU out of range")
	}

	return nil
}

func validateVLAN(vlan int, vlans []api.SystemNetworkVLAN) error {
	foundVLAN := false

	for _, v := range vlans {
		if v.ID == vlan {
			foundVLAN = true

			break
		}
	}

	if !foundVLAN {
		return fmt.Errorf("no vlan %d defined", vlan)
	}

	return nil
}

func validateAddress(address string) error {
	if address == "" {
		return errors.New("has empty address")
	}

	if address == "dhcp4" || address == "dhcp6" || address == "slaac" {
		return nil
	}

	addressishRegex := regexp.MustCompile(`^[.:[:xdigit:]]+(/\d+)?$`)
	if !addressishRegex.MatchString(address) {
		return fmt.Errorf("invalid IP address '%s'", address)
	}

	return nil
}

func validateRequiredForOnline(val string) error {
	if val != "" && val != "ipv6" && val != "ipv4" && val != "both" && val != "any" && val != "no" {
		return fmt.Errorf("invalid RequiredForOnline value '%s'", val)
	}

	return nil
}

func validateHwaddr(hwaddr string, requireValidMAC bool) error {
	if hwaddr == "" {
		return errors.New("has no MAC address")
	}

	if requireValidMAC {
		hwaddrhRegex := regexp.MustCompile(`^[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}:[[:xdigit:]]{2}$`)
		if !hwaddrhRegex.MatchString(hwaddr) {
			return fmt.Errorf("invalid MAC address '%s'", hwaddr)
		}
	}

	return nil
}
