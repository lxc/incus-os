from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemResources(install_image):
    test_name = "incusos-api-system-resources"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Perform a basic sanity check of the returned data.
        result = vm.APIRequest("/1.0/system/resources")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        keys = result["metadata"].keys()
        for key in ["cpu", "memory", "network", "storage"]:
            if key not in keys:
                raise IncusOSException(f"missing expected key {key} in returned resources")
