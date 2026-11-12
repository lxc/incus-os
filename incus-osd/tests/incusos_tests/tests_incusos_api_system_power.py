import json

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemPower(install_image):
    test_name = "incusos-api-system-power"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Get current power configuration.
        result = vm.APIRequest("/1.0/system/power")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Configuration should be empty.
        if result["metadata"]["config"] != {}:
            raise IncusOSException("wasn't expecting a populated power config")

        # Put the configuration back unchanged.
        result = vm.APIRequest("/1.0/system/power", method="PUT", body=json.dumps(result["metadata"]))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Send invalid scaling governor.
        result = vm.APIRequest("/1.0/system/power", method="PUT", body=json.dumps({"config": {"cpu": {"scaling_governor": "invalid"}}}))
        if result["error_code"] != 400:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Verify the invalid configuration is not saved.
        result = vm.APIRequest("/1.0/system/power")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["config"] != {}:
            raise IncusOSException("invalid config was applied")
