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
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/disk/by-id/(usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0|scsi-0QEMU_QEMU_CD-ROM_incus_boot--media) target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root", regex=True)
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
        vm.WaitSystemReady(incusos_version, source="/dev/disk/by-id/(usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0|scsi-0QEMU_QEMU_CD-ROM_incus_boot--media)")

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

        # Verify that LUKS encryption is bound to PCRs 7+11+15
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part9")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 7+11+15")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part10")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 7+11+15")

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

        # Verify that LUKS encryption is bound to PCRs 7+11+15
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part9")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 7+11+15")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part10")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 7+11+15")

def TestBaselineInstallNVME(install_image):
    test_name = "baseline-install-nvme"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.SetDeviceProperty("root", "io.bus=nvme")

        vm.WaitSystemReady(incusos_version, source="/dev/disk/by-id/(usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0|scsi-0QEMU_QEMU_CD-ROM_incus_boot--media)", target="/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_root")

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

        # Verify that LUKS encryption is bound to PCRs 7+11+15
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_root-part9")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 7+11+15")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_root-part10")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 7+11+15")

def TestBaselineInstallNVMEReadonlyImage(install_image):
    test_name = "baseline-install-nvme-readonly-image"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, readonly_install_image="true") as vm:
        vm.SetDeviceProperty("root", "io.bus=nvme")

        vm.WaitSystemReady(incusos_version, source="/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0", target="/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_root")

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

        # Verify that LUKS encryption is bound to PCRs 7+11+15
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_root-part9")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 7+11+15")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_root-part10")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 7+11+15")
