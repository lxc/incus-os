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
