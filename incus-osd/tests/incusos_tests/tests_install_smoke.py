from .incus_test_vm import IncusTestVM, util

def TestInstallDontRemoveInstallMedia(install_image):
    test_name = "dont-remove-install-media"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/(sdb|mapper/sr0) target=/dev/sda", regex=True)
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install but don't remove install media.
        vm.StopVM()

        # Start freshly installed IncusOS.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: install media detected, but the system is already installed; please remove USB/CDROM and reboot the system")

def TestBaselineInstall(install_image):
    test_name = "baseline-install"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, source="/dev/(sdb|mapper/sr0)")

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestBaselineInstallReadonlyImage(install_image):
    test_name = "baseline-install-readonly-image"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, readonly_install_image="true") as vm:
        vm.WaitSystemReady(incusos_version)

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestBaselineInstallNVME(install_image):
    test_name = "baseline-install-nvme"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.SetDeviceProperty("root", "io.bus=nvme")

        vm.WaitSystemReady(incusos_version, source="/dev/(sda|mapper/sr0)", target="/dev/nvme0n1")

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestBaselineInstallNVMEReadonlyImage(install_image):
    test_name = "baseline-install-nvme-readonly-image"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, readonly_install_image="true") as vm:
        vm.SetDeviceProperty("root", "io.bus=nvme")

        vm.WaitSystemReady(incusos_version, source="/dev/sda", target="/dev/nvme0n1")

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")
