import time

from .incus_test_vm import IncusTestVM, util

def TestBaselineUpgrade(install_image):
    test_name = "baseline-upgrade"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/sdb target=/dev/sda")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and expect an immediate upgrade.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        match = vm.WaitExpectedLog("incus-osd", "Downloading OS update version=(\\d+)", regex=True)
        new_version = match.group(1)
        vm.WaitExpectedLog("incus-osd", "Applying OS update version="+new_version)

        # Allow some time for the update to apply.
        time.sleep(30)

        # Wait for the system to automatically reboot after installing the upgrade.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready version="+new_version)
