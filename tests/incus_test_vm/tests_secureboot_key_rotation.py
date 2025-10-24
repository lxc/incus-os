from . import IncusTestVM, util

def TestSecureBootKeyRotation(install_image):
    test_name = "secure-boot-key-rotation"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Verify the new SecureBoot key isn't already present.
        result = vm.RunCommand("efi-readvar")
        if "CN=Incus OS - Secure Boot 2026 R1, O=Linux Containers" in str(result.stdout):
            raise Exception("new SecureBoot key is already present")

        # Apply the SecureBoot key update
        with open("secureboot-update.tar.gz", mode="rb") as update:
            result = vm.APIRequest("/1.0/debug/secureboot/:update", method="POST", body=update.read())
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        vm.WaitExpectedLog("incus-osd", "Appending certificate SHA256:2243C49FCF6F84FE670F100ECAFA801389DC207536CB9CA87AA2C062DDEBFDE5 to EFI variable db")
        vm.WaitExpectedLog("incus-osd", "Successfully updated EFI variable")

        # Make sure we can properly reboot with updated PCR7 value.
        vm.StopVM()
        vm.StartVM()

        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready release="+incusos_version)

        # Verify the new SecureBoot key is now present.
        result = vm.RunCommand("efi-readvar")
        if "CN=Incus OS - Secure Boot 2026 R1, O=Linux Containers" not in str(result.stdout):
            raise Exception("updated SecureBoot key not present after reboot")
