from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemProviderImages(install_image):
    test_name = "incusos-api-system-provider-images"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Get current provider configuration.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "images":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider 'images' shouldn't be registered")

def TestIncusOSAPISystemProviderOperationsCenter(install_image):
    test_name = "incusos-api-system-provider-operations-center"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"operations-center"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="operations-center")

        # Get current provider configuration.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "operations-center":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if not result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider 'operations-center' should be registered")

        # Attempting to register with the images provider should fail.
        result = vm.APIRequest("/1.0/system/provider", method="PUT", body="""{"config":{"name":"images"}}""")
        if result["status_code"] != 0 or result["error"] != "deregistration unsupported":
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify the provider registration hasn't actually been changed.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "operations-center":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if not result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider 'operations-center' should be registered")
