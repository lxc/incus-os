import time

from .incus_test_vm import IncusTestVM, util

def TestIncusOSAPISystem(install_image):
    test_name = "incusos-api-system"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Test top-level /1.0/system endpoint.
        result = vm.APIRequest("/1.0/system")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]) != 7:
            raise Exception("expected seven system endpoints")

        for endpoint in ["/1.0/system/logging", "/1.0/system/network", "/1.0/system/provider", \
            "/1.0/system/resources", "/1.0/system/security","/1.0/system/storage", "/1.0/system/update"]:
            if endpoint not in result["metadata"]:
                raise Exception(f"missing expected endpoint {endpoint}")

def TestIncusOSAPISystemPoweroff(install_image):
    test_name = "incusos-api-system-poweroff"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Command the system to poweroff.
        result = vm.APIRequest("/1.0/system/:poweroff", method="POST")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        time.sleep(5)

        # When the VM is powered off, attempting to stop it should raise an exception.
        try:
            vm.StopVM()
        except:
            return
        else:
            raise Exception("VM didn't power itself off")

def TestIncusOSAPISystemReboot(install_image):
    test_name = "incusos-api-system-reboot"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Command the system to reboot.
        result = vm.APIRequest("/1.0/system/:reboot", method="POST")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        time.sleep(5)

        # Wait for the system to come back up.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready release="+incusos_version)

        # Get journal entries from the prior boot, which can only happen if the VM successfully rebooted.
        result = vm.APIRequest("/1.0/debug/log?unit=incus-osd&boot=-1")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))
