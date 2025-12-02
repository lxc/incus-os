import json

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemLogging(install_image):
    test_name = "incusos-api-system-logging"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Get current logging configuration.
        result = vm.APIRequest("/1.0/system/logging")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["syslog"]["address"] != "" or \
            result["metadata"]["config"]["syslog"]["protocol"] != "" or \
            result["metadata"]["config"]["syslog"]["log_format"] != "":
            raise IncusOSException("wasn't expecting a populated syslog config")

        # Change the logging configuration.
        result["metadata"]["config"]["syslog"]["address"] = "127.0.0.1"
        result["metadata"]["config"]["syslog"]["protocol"] = "tcp"
        result["metadata"]["config"]["syslog"]["log_format"] = "rfc5424"

        result = vm.APIRequest("/1.0/system/logging", method="PUT", body=json.dumps(result["metadata"]))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify the changes.
        result = vm.APIRequest("/1.0/system/logging")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["syslog"]["address"] != "127.0.0.1" or \
            result["metadata"]["config"]["syslog"]["protocol"] != "tcp" or \
            result["metadata"]["config"]["syslog"]["log_format"] != "rfc5424":
            raise IncusOSException("returned syslog config was incorrect")
