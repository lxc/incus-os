import json
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def _checkNetworkConnectivity(vm):
    vm.RunCommand("curl", "linuxcontainers.org")

def TestIncusOSAPISystemNetworkDefaults(install_image):
    test_name = "incusos-api-system-network-defaults"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Allow the network state to settle
        time.sleep(5)

        # Get current network configuration and state
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

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

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and verify error about configuring the network.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Bringing up the network")
        vm.WaitExpectedLog("incus-osd", "timed out waiting for configured network interfaces, missing interface(s): eth0 (00:11:22:33:44:55), eth1 (ff:ee:dd:cc:bb:aa)")

        # We shouldn't see anything about the system being ready.
        vm.LogDoesntContain("incus-osd", "System is ready version="+incusos_version)

def TestIncusOSAPISystemNetworkRollback(install_image):
    test_name = "incusos-api-system-network-rollback"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Allow the network state to settle
        time.sleep(5)

        # Get current network configuration
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

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
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Let the new network configuration settle
        time.sleep(10)

        # Get the updated network configuration and verify connectivity still works
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

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

        vm.WaitExpectedLog("incus-osd", "Rolling back network configuration to prior known-good state")
        vm.LogDoesntContain("incus-osd", "Failed to roll back network configuration")

        # Verify the configuration has been rolled back as expected
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

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
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Let the new network configuration settle
        time.sleep(10)

        # Get the updated network configuration and verify connectivity still works
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["config"]["interfaces"][0]["addresses"]) != 1 or \
            result["metadata"]["config"]["interfaces"][0]["addresses"][0] != "dhcp4":
            raise IncusOSException("expected interface to be configured with only dhcp4")

        if len(result["metadata"]["state"]["interfaces"]["enp5s0"]["addresses"]) != 1:
            raise IncusOSException("expected interface enp5s0 to have exactly one address")

        _checkNetworkConnectivity(vm)

        result = vm.APIRequest("/1.0/system/network/:confirm", method="POST")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Sleep an additional 45 seconds to let the confirmation timeout elapse and verify the changes weren't rolled back
        time.sleep(45)

        # Verify the configuration wasn't rolled back
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

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
