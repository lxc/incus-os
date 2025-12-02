from .incus_test_vm import IncusTestVM, IncusOSException, util

### TODO -- There's not really much actual testing of the individual services yet.

def TestIncusOSAPIServices(install_image):
    test_name = "incusos-api-services"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Test top-level /1.0/services endpoint.
        result = vm.APIRequest("/1.0/services")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]) == 0:
            raise IncusOSException("expected at least one services endpoint")

        # Do a simple query of each service.
        for service in result["metadata"]:
            result = vm.APIRequest(service)
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))
