from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPI(install_image):
    test_name = "incusos-api"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Test top-level /1.0 endpoint.
        result = vm.APIRequest("/1.0")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["environment"]["os_name"] != os_name:
            raise IncusOSException("unexpected OS Name: " + result["metadata"]["environment"]["os_name"])

        if result["metadata"]["environment"]["os_version"] != os_version:
            raise IncusOSException("unexpected OS Version: " + result["metadata"]["environment"]["os_version"])

        # Test the license information.
        result = vm.APIRequest("/1.0/licenses")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        for name in ["local:incus-osd", "debian:systemd", "license:GPL-2"]:
            if "/1.0/licenses/" + name not in result["metadata"]:
                raise IncusOSException("missing license entry: " + name)

        result = vm.APIRequest("/1.0/licenses/local:incus-osd")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["type"] != "local" or result["metadata"]["name"] != "incus-osd":
            raise IncusOSException("unexpected license entry: " + result["metadata"]["type"] + ":" + result["metadata"]["name"])

        if "Apache License" not in result["metadata"]["content"]:
            raise IncusOSException("incus-osd license doesn't appear to be the Apache license")

        result = vm.APIRequest("/1.0/licenses/debian:does-not-exist")
        if result["error_code"] != 404:
            raise IncusOSException("unexpected error code %d for a missing license entry" % result["error_code"])
