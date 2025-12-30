import json

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestInstallSecureBootDisabled(install_image):
    test_name = "secureboot-disabled"
    test_seed = {
        "install.json": """{"security":{"missing_secure_boot":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)
    util._remove_secureboot_keys(test_image)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Should see a log message about SecureBoot being disabled
        vm.WaitExpectedLog("incus-osd", "Degraded security state: Secure Boot is disabled")

        # Verify no SecureBoot EFI variables are set
        result = vm.RunCommand("efi-readvar")
        if "Variable PK has no entries" not in str(result.stdout) or "Variable db has no entries" not in str(result.stdout):
            raise IncusOSException("SecureBoot EFI variables shouldn't be populated")

        # Verify that LUKS encryption is bound to PCRs 4+7+11
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/sda9")
        if "tpm2-hash-pcrs:   4+7" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 4+7+11")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/sda10")
        if "tpm2-hash-pcrs:   4+7" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 4+7+11")

        # Verify Secure Boot being disabled is reflected in security state
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["state"]["system_state_is_trusted"]:
            raise IncusOSException("expected to see system state is untrusted")

        # Set a different encryption recovery key.
        result["metadata"]["config"]["encryption_recovery_keys"][0] = "foo-bar-biz-1234"
        result = vm.APIRequest("/1.0/system/security", method="PUT", body=json.dumps(result["metadata"]))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))
