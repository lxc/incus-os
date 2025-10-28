from .incus_test_vm import IncusTestVM, util

def TestIncusOSAPI(install_image):
    test_name = "incusos-api"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Test top-level /1.0 endpoint.
        result = vm.APIRequest("/1.0")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["environment"]["os_name"] != "IncusOS":
            raise Exception("unexpected OS Name: " + result["metadata"]["environment"]["os_name"])

        if result["metadata"]["environment"]["os_version"] != incusos_version:
            raise Exception("unexpected OS Version: " + result["metadata"]["environment"]["os_version"])
