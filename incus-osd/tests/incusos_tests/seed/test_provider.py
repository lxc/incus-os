from ..incus_test_vm import IncusTestVM, IncusOSException, util

def TestSeedProvider(install_image):
    test_name = "seed-provider"
    test_seed = {
        "install.json": "{}",
        "provider.json": """{"name":"debug","config":{"myKey":"testing"}}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and verify the provider seed was properly applied.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "System is ready version="+os_version)

        # We shouldn't see anything about loading an application from the debug provider.
        vm.LogDoesntContain("incus-osd", "Downloading application update application=")

        # Check that we get back the expected provider configuration.
        result = vm.APIRequest("/1.0/system/provider", use_unix_socket=True)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["config"]["name"] != "debug":
            raise IncusOSException("expected provider name to be 'debug'")

        if result["metadata"]["config"]["config"]["myKey"] != "testing":
            raise IncusOSException("expected provider configuration key 'myKey' to be 'testing'")

def TestSeedProviderInvalid(install_image):
    test_name = "seed-provider-invalid"
    test_seed = {
        "install.json": "{}",
        "provider.json": """{"name":"bizbaz"}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and check for the expected error loading the provider.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "unknown provider \"bizbaz\"")
