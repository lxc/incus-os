import json
import os
import time

from .incus_test_vm import IncusTestNetwork, IncusTestVM, IncusOSException, util

def _checkNetworkConnectivity(vm):
    IMAGES_SERVER = os.getenv("IMAGES_SERVER", "https://images.linuxcontainers.org")

    vm.RunCommand("curl", IMAGES_SERVER)

def TestIncusOSAPISystemNetworkDefaults(install_image):
    test_name = "incusos-api-system-network-defaults"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Allow the network state to settle
        time.sleep(5)

        # Get current network configuration and state
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        config = result["metadata"]["config"]

        if config["time"]["timezone"] != "UTC":
            raise IncusOSException("default timezone isn't UTC: " + config["time"]["timezone"])

        if len(config["interfaces"]) != 1:
            raise IncusOSException("expected exactly one interface to be configured")

        if config["interfaces"][0]["name"] != "enp5s0":
            raise IncusOSException("unexpected interface name: " + config["interfaces"][0]["name"])

        if "dhcp4" not in config["interfaces"][0]["addresses"]:
            raise IncusOSException("expected interface to be configured with dhcp4")

        if "slaac" not in config["interfaces"][0]["addresses"]:
            raise IncusOSException("expected interface to be configured with slaac")

        interfaces = result["metadata"]["state"]["interfaces"]

        if "enp5s0" not in interfaces:
            raise IncusOSException("expected interface enp5s0 to exist")

        if interfaces["enp5s0"]["type"] != "interface":
            raise IncusOSException("expected interface enp5s0 type to be an interface")

        if len(interfaces["enp5s0"]["addresses"]) != 2:
            raise IncusOSException("expected interface enp5s0 to have exactly two addresses")

        if interfaces["enp5s0"]["mtu"] != 1500:
            raise IncusOSException("expected interface enp5s0 MTU to be 1500")

        if "management" not in interfaces["enp5s0"]["roles"] or "cluster" not in interfaces["enp5s0"]["roles"]:
            raise IncusOSException("expected interface enp5s0 to have the management and cluster roles")

        # Perform a simple connectivity test
        _checkNetworkConnectivity(vm)

def TestIncusOSAPISystemNetworkBadMAC(install_image):
    test_name = "incusos-api-system-network-bad-mac"
    test_seed = {
        "install.json": "{}",
        "network.json": """{"interfaces":[{"addresses":["dhcp4"],"hwaddr":"00:11:22:33:44:55","name":"eth0"},{"addresses":["slaac"],"hwaddr":"ff:ee:dd:cc:bb:aa","name":"eth1"}]}""",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and verify error about configuring the network.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Bringing up the network")
        vm.WaitExpectedLog("incus-osd", "unable to determine maximum MTU for 00:11:22:33:44:55")

        # We shouldn't see anything about the system being ready.
        vm.LogDoesntContain("incus-osd", "System is ready version="+os_version)

def TestIncusOSAPISystemNetworkRollback(install_image):
    test_name = "incusos-api-system-network-rollback"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Allow the network state to settle
        time.sleep(5)

        # Get current network configuration
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        networkCfg = result["metadata"]

        ## Test invalid confirmation timeouts
        networkCfg["config"]["confirmation_timeout"] = "bizbaz"

        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] == 200:
            raise IncusOSException("unexpected success setting an invalid confirmation timeout")

        if result["error"] != "invalid confirmation timeout provided: time: invalid duration \"bizbaz\"":
            raise IncusOSException("unexpected error message: " + result["error"])

        networkCfg["config"]["confirmation_timeout"] = "-1m"

        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] == 200:
            raise IncusOSException("unexpected success setting an invalid confirmation timeout")

        if result["error"] != "confirmation timeout must be greater than zero":
            raise IncusOSException("unexpected error message: " + result["error"])

        ## Test rollback functionality if the changes are not confirmed
        networkCfg["config"]["confirmation_timeout"] = "45s"
        networkCfg["config"]["interfaces"][0]["addresses"] = ["dhcp4"]

        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Let the new network configuration settle
        time.sleep(10)

        # Get the updated network configuration and verify connectivity still works
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if len(result["metadata"]["config"]["interfaces"][0]["addresses"]) != 1 or \
            result["metadata"]["config"]["interfaces"][0]["addresses"][0] != "dhcp4":
            raise IncusOSException("expected interface to be configured with only dhcp4")

        if len(result["metadata"]["state"]["interfaces"]["enp5s0"]["addresses"]) != 1:
            raise IncusOSException("expected interface enp5s0 to have exactly one address")

        _checkNetworkConnectivity(vm)

        # Can't apply more than one pending network configuration at a time
        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] == 200:
            raise IncusOSException("unexpected success applying a second network configuration while one is still pending")

        if result["error"] != "a pending network configuration must first be confirmed before a new configuration can be applied":
            raise IncusOSException("unexpected error message: " + result["error"])

        # Sleep an additional 45 seconds to let the confirmation timeout elapse and revert the network configuration
        time.sleep(45)

        vm.WaitExpectedLog("incus-osd", "Timeout expired, rolling back network configuration to prior known-good state")
        vm.LogDoesntContain("incus-osd", "Failed to roll back network configuration")

        # Verify the configuration has been rolled back as expected
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if "dhcp4" not in result["metadata"]["config"]["interfaces"][0]["addresses"]:
            raise IncusOSException("expected interface to be configured with dhcp4")

        if "slaac" not in result["metadata"]["config"]["interfaces"][0]["addresses"]:
            raise IncusOSException("expected interface to be configured with slaac")

        if len(result["metadata"]["state"]["interfaces"]["enp5s0"]["addresses"]) != 2:
            raise IncusOSException("expected interface enp5s0 to have exactly two addresses")

        _checkNetworkConnectivity(vm)

        ## Next, test automatically rolling back a bad network configuration without needing to wait for the timeout to expire
        networkCfg["config"]["interfaces"][0]["addresses"] = ["dhcp4", "bizbaz"]

        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] == 200:
            raise IncusOSException("unexpected success setting an invalid network configuration")

        if result["error"] != "interface 0 address 1 invalid IP address 'bizbaz', must provide a CIDR mask":
            raise IncusOSException("unexpected error message: " + result["error"])

        # Restore the correct new IP configuration for the interface
        networkCfg["config"]["interfaces"][0]["addresses"] = ["dhcp4"]

        # Sleep 10 seconds to allow the network configuration to properly revert and settle
        time.sleep(10)

        vm.WaitExpectedLog("incus-osd", "Invalid network configuration detected, rolling back to prior known-good state")

        # Verify the configuration has been rolled back as expected
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if "dhcp4" not in result["metadata"]["config"]["interfaces"][0]["addresses"]:
            raise IncusOSException("expected interface to be configured with dhcp4")

        if "slaac" not in result["metadata"]["config"]["interfaces"][0]["addresses"]:
            raise IncusOSException("expected interface to be configured with slaac")

        if len(result["metadata"]["state"]["interfaces"]["enp5s0"]["addresses"]) != 2:
            raise IncusOSException("expected interface enp5s0 to have exactly two addresses")

        _checkNetworkConnectivity(vm)

        ## Finally, test that confirming the changes persists the new network configuration
        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Let the new network configuration settle
        time.sleep(10)

        # Get the updated network configuration and verify connectivity still works
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if len(result["metadata"]["config"]["interfaces"][0]["addresses"]) != 1 or \
            result["metadata"]["config"]["interfaces"][0]["addresses"][0] != "dhcp4":
            raise IncusOSException("expected interface to be configured with only dhcp4")

        if len(result["metadata"]["state"]["interfaces"]["enp5s0"]["addresses"]) != 1:
            raise IncusOSException("expected interface enp5s0 to have exactly one address")

        _checkNetworkConnectivity(vm)

        result = vm.APIRequest("/1.0/system/network/:confirm", method="POST")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Sleep an additional 45 seconds to let the confirmation timeout elapse and verify the changes weren't rolled back
        time.sleep(45)

        # Verify the configuration wasn't rolled back
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if len(result["metadata"]["config"]["interfaces"][0]["addresses"]) != 1 or \
            result["metadata"]["config"]["interfaces"][0]["addresses"][0] != "dhcp4":
            raise IncusOSException("expected interface to be configured with only dhcp4")

        if len(result["metadata"]["state"]["interfaces"]["enp5s0"]["addresses"]) != 1:
            raise IncusOSException("expected interface enp5s0 to have exactly one address")

        # Verify there's no pending network configuration
        result = vm.APIRequest("/1.0/system/network/:confirm", method="POST")
        if result["status_code"] == 200:
            raise IncusOSException("unexpected success confirming a non-existent network configuration change")

        if result["error"] != "no network configuration is pending a confirmation":
            raise IncusOSException("unexpected error message: " + result["error"])

def TestIncusOSAPISystemNetworkConfigInterfaces(install_image):
    test_name = "incusos-api-system-network-config-interfaces"
    test_seed = {
        "install.json": "{}"
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestNetwork() as network:
        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AddDevice("eth1", "nic", "network="+network.name)

            vm.WaitSystemReady(os_version)

            # Get current network configuration.
            result = vm.APIRequest("/1.0/system/network")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            interfaces = result["metadata"]["state"]["interfaces"]

            if "enp5s0" not in interfaces or "enp6s0" not in interfaces:
                raise IncusOSException("expected interfaces enp5s0 and enp6s0 to exist")

            enp5MAC = '"' + interfaces["enp5s0"]["hwaddr"] + '"'
            enp6MAC = '"' + interfaces["enp6s0"]["hwaddr"] + '"'

            # Apply a new network configuration that changes names and roles.
            result = vm.APIRequest("/1.0/system/network", method="PUT", body="""{"config":{"time":{"timezone":"UTC"},"interfaces":[{"addresses":["dhcp4","slaac"],"hwaddr":""" + enp5MAC + ""","name":"management","required_for_online":"both","roles":["management","cluster","storage"]},{"hwaddr":""" + enp6MAC + ""","name":"vm","roles":["instances"]}]}}""")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            # Get the updated network configuration.
            result = vm.APIRequest("/1.0/system/network")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            interfaces = result["metadata"]["state"]["interfaces"]

            if "management" not in interfaces or "vm" not in interfaces:
                raise IncusOSException("expected interfaces management and vm to exist")

            if len(interfaces["management"]["addresses"]) != 2:
                raise IncusOSException("expected management interface to have two addresses")

            if "management" not in interfaces["management"]["roles"] or "cluster" not in interfaces["management"]["roles"] or "storage" not in interfaces["management"]["roles"] or len(interfaces["management"]["roles"]) != 3:
                raise IncusOSException("missing expected roles for management interface: " + str(interfaces["management"]["roles"]))

            if "addresses" in interfaces["vm"]:
                raise IncusOSException("expected vm interface not to have any addresses")

            if "instances" not in interfaces["vm"]["roles"] or len(interfaces["vm"]["roles"]) != 1:
                raise IncusOSException("missing expected roles for vm interface: " + str(interfaces["vm"]["roles"]))

def TestIncusOSAPISystemNetworkBond(install_image):
    test_name = "incusos-api-system-network-bond"
    test_seed = {
        "install.json": "{}",
        "network.json": """{"bonds":[{"addresses":["dhcp4","slaac"],"members":["enp5s0","enp6s0"],"mode":"active-backup","mtu":1450,"name":"bond0"}]}""",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestNetwork(dns_mode="none", mtu=9000) as bond_network:
        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            # Configure two NICs for creation of a bond
            vm.AddDevice("eth0", "nic", "network="+bond_network.name)
            vm.AddDevice("eth1", "nic", "network="+bond_network.name)

            vm.WaitSystemReady(os_version)

            # Allow the network state to settle
            time.sleep(5)

            # Get current network state
            result = vm.APIRequest("/1.0/system/network")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            interfaces = result["metadata"]["state"]["interfaces"]

            if len(interfaces) != 1 or "bond0" not in interfaces:
                raise IncusOSException("expected bond bond0 to exist")

            if interfaces["bond0"]["type"] != "bond":
                raise IncusOSException("expected bond bond0 type to be a bond")

            if len(interfaces["bond0"]["addresses"]) != 2:
                raise IncusOSException("expected bond bond0 to have exactly two addresses")

            if interfaces["bond0"]["mtu"] != 1450:
                raise IncusOSException("expected bond bond0 MTU to be 1450")

            if len(interfaces["bond0"]["members"]) != 2:
                raise IncusOSException("expected bond bond0 to have exactly two members")

            members = []
            for name, member in interfaces["bond0"]["members"].items():
                members.append(name)

                if member["mtu"] != 9000:
                    raise IncusOSException("expected bond member " + name + " to have a MTU of 9000")

            # Perform an initial simple connectivity test
            _checkNetworkConnectivity(vm)

            # Disable the first bond member and ensure we still have connectivity
            vm.RunCommand("networkctl", "down", members[0])
            time.sleep(1)
            _checkNetworkConnectivity(vm)

            # Disable the second bond member and verify we don't have connectivity
            vm.RunCommand("networkctl", "down", members[1])
            time.sleep(1)
            try:
                _checkNetworkConnectivity(vm)
            except:
                # Except an exception to have been raised when curl fails
                pass
            else:
                raise IncusOSException("unexpected network connectivity with both bond members disabled")

            # Re-enable the first bond member and check that connectivity has returned
            vm.RunCommand("networkctl", "up", members[0])
            time.sleep(1)
            _checkNetworkConnectivity(vm)

def TestIncusOSAPISystemNetworkComplex(install_image):
    test_name = "incusos-api-system-network-complex"
    test_seed = {
        "install.json": "{}",
        "network.json": """{"interfaces":[{"addresses":["dhcp4","slaac"],"hwaddr":"enp5s0","mtu":1789,"name":"enp5s0","required_for_online":"both"}],"bonds":[{"addresses":["192.168.200.200/24"],"lldp":true,"members":["enp6s0","enp7s0"],"mode":"active-backup","mtu":9000,"name":"backbone","required_for_online":"no","roles":["management","instances"],"routes":[{"to":"0.0.0.0/0","via":"192.168.200.1"}],"vlan_tags":[101,102,105]},{"addresses":["dhcp4"],"lldp":true,"members":["enp8s0","enp9s0"],"mode":"active-backup","mtu":8500,"name":"dmz","required_for_online":"no","roles":["instances"]}],"vlans":[{"addresses":["192.168.100.100/24"],"id":110,"mtu":2345,"name":"internal","parent":"backbone","roles":["cluster"]}]}""",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestNetwork(dns_mode="none", mtu=9000) as bond_network1:
        with IncusTestNetwork(dns_mode="none", mtu=9000) as bond_network2:
            with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
                # Configure four NICs for creation of bonds
                vm.AddDevice("eth1", "nic", "network="+bond_network1.name)
                vm.AddDevice("eth2", "nic", "network="+bond_network1.name)
                vm.AddDevice("eth3", "nic", "network="+bond_network2.name)
                vm.AddDevice("eth4", "nic", "network="+bond_network2.name)

                vm.WaitSystemReady(os_version)

                # Allow the network state to settle
                time.sleep(5)

                # Perform an initial simple connectivity test
                _checkNetworkConnectivity(vm)

                # Get current network state
                result = vm.APIRequest("/1.0/system/network")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                interfaces = result["metadata"]["state"]["interfaces"]

                # Check that all four reported interfaces are properly configured
                if len(interfaces) != 4:
                    raise IncusOSException("expected four interfaces to exist")

                if "enp5s0" not in interfaces:
                    raise IncusOSException("interface enp5s0 doesn't exist")

                if len(interfaces["enp5s0"]["addresses"]) != 2:
                    raise IncusOSException("expected interface enp5s0 to have two addresses")

                if interfaces["enp5s0"]["type"] != "interface":
                    raise IncusOSException("interface enp5s0 type isn't interface")

                if interfaces["enp5s0"]["mtu"] != 1789:
                    raise IncusOSException("interface enp5s0 MTU wasn't 1789")

                if "internal" not in interfaces:
                    raise IncusOSException("vlan internal doesn't exist")

                if len(interfaces["internal"]["addresses"]) != 1:
                    raise IncusOSException("expected vlan internal to have one address")

                if interfaces["internal"]["addresses"][0] != "192.168.100.100":
                    raise IncusOSException("vlan internal has incorrect address")

                if interfaces["internal"]["type"] != "vlan":
                    raise IncusOSException("vlan internal type isn't vlan")

                if interfaces["internal"]["mtu"] != 2345:
                    raise IncusOSException("vlan internal MTU wasn't 2345")

                if len(interfaces["internal"]["roles"]) != 1:
                    raise IncusOSException("expected vlan internal to have one role")

                if interfaces["internal"]["roles"][0] != "cluster":
                    raise IncusOSException("vlan internal has incorrect role")

                if "backbone" not in interfaces:
                    raise IncusOSException("bond backbone doesn't exist")

                if len(interfaces["backbone"]["addresses"]) != 1:
                    raise IncusOSException("expected bond backbone to have one address")

                if interfaces["backbone"]["addresses"][0] != "192.168.200.200":
                    raise IncusOSException("bond backbone has incorrect address")

                if interfaces["backbone"]["type"] != "bond":
                    raise IncusOSException("bond backbone type isn't bond")

                if len(interfaces["backbone"]["members"]) != 2:
                    raise IncusOSException("bond backbone doesn't have two members")

                if interfaces["backbone"]["mtu"] != 9000:
                    raise IncusOSException("bond backbone MTU wasn't 9000")

                if len(interfaces["backbone"]["roles"]) != 2:
                    raise IncusOSException("expected bond backbone to have two roles")

                if "management" not in interfaces["backbone"]["roles"] or "instances" not in interfaces["backbone"]["roles"]:
                    raise IncusOSException("bond backbone has incorrect roles")

                if "dmz" not in interfaces:
                    raise IncusOSException("bond dmz doesn't exist")

                if len(interfaces["dmz"]["addresses"]) != 1:
                    raise IncusOSException("expected bond dmz to have one address")

                if interfaces["dmz"]["type"] != "bond":
                    raise IncusOSException("bond dmz type isn't bond")

                if len(interfaces["dmz"]["members"]) != 2:
                    raise IncusOSException("bond dmz doesn't have two members")

                if interfaces["dmz"]["mtu"] != 8500:
                    raise IncusOSException("bond dmz MTU wasn't 8500")

                if len(interfaces["dmz"]["roles"]) != 1:
                    raise IncusOSException("expected bond dmz to have one role")

                if interfaces["dmz"]["roles"][0] != "instances":
                    raise IncusOSException("bond dmz has incorrect role")

                # Check VLANs
                output = vm.RunCommand("ip", "-d", "link", "show", "dev", "internal")
                if "vlan protocol 802.1Q id 110 " not in output.stdout.decode("utf-8"):
                    raise IncusOSException("VLAN 110 not present on interface internal")

                for id in ["101", "102", "105", "110"]:
                    output = vm.RunCommand("bridge", "vlan", "show", "dev", "_bbackbone", "vid", id)
                    if "_bbackbone" not in output.stdout.decode("utf-8"):
                        raise IncusOSException("VLAN " + id + " not present on bond backbone")
