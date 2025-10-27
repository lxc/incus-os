import json

from .incus_test_vm import IncusTestVM, util

def TestIncusOSAPISystemSecurity(install_image):
    test_name = "incusos-api-system-security"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Get current security configuration and state.
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["config"]["encryption_recovery_keys"]) != 1:
            raise Exception("expected exactly one encryption recovery key")

        if not result["metadata"]["state"]["encryption_recovery_keys_retrieved"]:
            raise Exception("invalid encryption_recovery_keys_retrieved state")

        if len(result["metadata"]["state"]["encrypted_volumes"]) != 2:
            raise Exception("expected exactly two encrypted volumes")

        # Add a second encryption recovery key.
        result["metadata"]["config"]["encryption_recovery_keys"].append("foo-bar-biz")
        result = vm.APIRequest("/1.0/system/security", method="PUT", body=json.dumps(result["metadata"]))
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify two encryption recovery keys are present.
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["config"]["encryption_recovery_keys"]) != 2:
            raise Exception("expected exactly two encryption recovery keys")

        if "foo-bar-biz" not in result["metadata"]["config"]["encryption_recovery_keys"]:
            raise Exception("new encryption key isn't present")

def TestIncusOSAPISystemSecurityTPMRebind(install_image):
    test_name = "incusos-api-system-security-tpm-rebind"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # By default, forcing a TPM rebind will fail unless the system is in a changed state.
        result = vm.APIRequest("/1.0/system/security/:tpm-rebind", method="POST")
        if result["status_code"] != 0 or result["error"] != "refusing to reset TPM encryption bindings because current state can unlock all volumes":
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # This is a sledgehammer approach, but fine for the test. :)
        vm.RunCommand("tpm2_clear")

        # Now we expect TPM rebinding to work.
        result = vm.APIRequest("/1.0/system/security/:tpm-rebind", method="POST")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # The VM will automatically reboot, but since we just smashed things with a hammer it will hang
        # waiting for an encryption recovery key to be provided. So, nothing more to do in this test.
