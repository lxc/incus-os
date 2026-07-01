import os
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemProviderImages(install_image):
    test_name = "incusos-api-system-provider-images"
    test_seed = {
        "install.json": "{}"
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Get current provider configuration.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

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

    # If the default images server has been overridden, assume we're testing against a local build of IncusOS
    # and grab the local root CA certificate so Operations Center can properly verify the signed json metadata.
    IMAGES_SERVER = os.getenv("IMAGES_SERVER")
    if IMAGES_SERVER is not None:
        with open("incus-osd/certs/files/root-E1.crt") as f:
            contents = f.read().replace("\n", "\\n")

            # Configure the Operations Center seed to point to the local images server
            test_seed["operations-center.json"] = '{"preseed":{"system_updates":{"source":"' + IMAGES_SERVER + '/os","signature_verification_root_ca":"' + contents + '"}}}'

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="operations-center")

        vm.WaitExpectedLog("incus-osd", "Server successfully deregistered from the 'images' provider")
        vm.WaitExpectedLog("incus-osd", "Server successfully registered with the 'operations-center' provider")

        # Get current provider configuration.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "operations-center":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if not result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider 'operations-center' should be registered")

        # Attempting to register with the images provider should fail.
        result = vm.APIRequest("/1.0/system/provider", method="PUT", body="""{"config":{"name":"images"}}""")
        if result["status_code"] != 0 or result["error"] != "deregistration unsupported":
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Verify the provider registration hasn't actually been changed.
        result = vm.APIRequest("/1.0/system/provider")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "operations-center":
            raise IncusOSException("unexpected provider: " + result["metadata"]["config"]["name"])

        if not result["metadata"]["state"]["registered"]:
            raise IncusOSException("provider 'operations-center' should be registered")

        # Trigger a provisioning update refresh, and wait until it finishes.
        result = vm.APIRequest("/1.0/provisioning/updates/:refresh?wait=true", method="POST", use_os_proxy=False)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        found_update = False

        # Verify that we get back an expected update with the same version that the system is currently running.
        result = vm.APIRequest("/1.0/provisioning/updates?recursion=1", use_os_proxy=False)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        for update in result["metadata"]:
            if update["version"] == os_version and update["update_status"] == "ready":
                found_update = True

        if not found_update:
            raise IncusOSException("Operations Center failed to fetch available updates")

        # Install the debug application via local Operations Center
        result = vm.APIRequest("/1.0/applications", method="POST", body="""{"name":"debug"}""")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        vm.WaitExpectedLog("incus-osd", "Downloading application update application=debug channel=stable version="+os_version)
        vm.WaitExpectedLog("incus-osd", "Initializing application name=debug version="+os_version)
