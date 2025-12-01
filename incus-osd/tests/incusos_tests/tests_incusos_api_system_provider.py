from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemProvider(install_image):
    test_name = "incusos-api-system-provider"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"operations-center"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Setup with Operations Center, so we can actually test changing provider registration.
        vm.WaitSystemReady(incusos_version, application="operations-center")

        # Get current provider configuration.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "images":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider shouldn't be registered")

        # Attempting to register with the images provider should fail.
        result = vm.APIRequest("/1.0/system/provider", method="PUT", body="""{"config":{"name":"images"}}""")
        if result["status_code"] != 0 or result["error"] != "registration unsupported":
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Try to register with Operations Center; this will fail due to server-side TLS certificate errors.
        result = vm.APIRequest("/1.0/system/provider", method="PUT", body="""{"config":{"name":"operations-center","config":{"server_url":"https://localhost:8443","server_token":"foobar"}}}""")
        if result["status_code"] != 0 or """Post \"https://localhost:8443/1.0/provisioning/servers?token=foobar\": tls: failed to verify certificate:""" not in result["error"]:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify the provider registration hasn't actually been changed.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "images":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider shouldn't be registered")
