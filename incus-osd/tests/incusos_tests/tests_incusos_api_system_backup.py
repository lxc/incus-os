import json
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemBackup(install_image):
    test_name = "incusos-api-system-backup"
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
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Set an initial hostname and timezone
        networkCfg = result["metadata"]
        networkCfg["config"]["dns"] = {"hostname": "incusos1"}
        networkCfg["config"]["time"] = {"timezone": "America/Chicago"}

        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Allow the network state to settle
        time.sleep(5)

        # Create an OS-level backup
        backup_archive = vm.APIRequest("/1.0/system/:backup", method="POST", return_raw_content=True)

        # Change the hostname and timezone
        networkCfg["config"]["dns"]["hostname"] = "incusos2"
        networkCfg["config"]["time"]["timezone"] = "America/Denver"

        result = vm.APIRequest("/1.0/system/network", method="PUT", body=json.dumps(networkCfg))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Allow the network state to settle
        time.sleep(5)

        # Verify the hostname and timezone have changed
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["config"]["dns"]["hostname"] != "incusos2":
            raise IncusOSException("unexpected hostname: " + result["metadata"]["config"]["dns"]["hostname"])

        if result["metadata"]["config"]["time"]["timezone"] != "America/Denver":
            raise IncusOSException("unexpected timezone: " + result["metadata"]["config"]["time"]["timezone"])

        # Restore the OS backup
        result = vm.APIRequest("/1.0/system/:restore", method="POST", body=backup_archive, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Sleep a bit to allow the restoration to complete and then wait for the VM to reboot
        time.sleep(15)
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Allow the network state to settle
        time.sleep(5)

        # Get the network configuration and verify the prior values were restored from the backup archive
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["config"]["dns"]["hostname"] != "incusos1":
            raise IncusOSException("unexpected hostname: " + result["metadata"]["config"]["dns"]["hostname"])

        if result["metadata"]["config"]["time"]["timezone"] != "America/Chicago":
            raise IncusOSException("unexpected timezone: " + result["metadata"]["config"]["time"]["timezone"])
