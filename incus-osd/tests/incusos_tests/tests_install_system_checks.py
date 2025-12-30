import os
import subprocess
import tempfile

from .incus_test_vm import IncusTestVM, util

def TestInstallNoSeed(install_image):
    test_name = "no-seed"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: source device '/dev/sdb' is too small \\(.+GiB\\), must be at least 50GiB", regex=True)

def TestInstallNoSeedReadonlyImage(install_image):
    test_name = "no-seed-readonly-image"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, readonly_install_image="true") as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: unable to begin install from read-only device '/dev/sdb' without seed configuration")

def TestInstallTooManyTargets(install_image):
    test_name = "too-many-targets"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "System check error: no target configuration provided, and didn't find exactly one install device")

def TestInstallDriveTooSmall(install_image):
    test_name = "drive-too-small"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, root_size="10GiB") as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: target device '/dev/sda' is too small (10.00GiB), must be at least 50GiB")

def TestInstallDriveWithGPT(install_image):
    test_name = "drive-with-gpt"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_disk1"}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        # Truncate the disk image file to 50GiB and setup a single GPT partition.
        disk_img.truncate(50*1024*1024*1024)
        subprocess.run(["/sbin/sgdisk", "-n", "1", disk_img.name], capture_output=True, check=True)

        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "a partition table already exists on device '/dev/sdb', and `ForceInstall` from install configuration isn't true")

def TestInstallCantDisableSBandTPM(install_image):
    test_name = "cant-disable-sb-and-tpm"
    test_seed = {
        "install.json": """{"security":{"missing_tpm":true,"missing_secure_boot":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: install seed cannot enable both Secure Boot and TPM degraded security options")

def TestInstallNoSWTPMwithTPM(install_image):
    test_name = "no-swtpm-with-tpm"
    test_seed = {
        "install.json": """{"security":{"missing_tpm":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: a physical TPM was found, but install seed wants to configure a swtpm-backed TPM")

def TestInstallNoDisabledSBwithSB(install_image):
    test_name = "no-disabled-sb-with-sb"
    test_seed = {
        "install.json": """{"security":{"missing_secure_boot":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: Secure Boot is enabled, but install seed expects it to be disabled")

def TestInstallNoTPMNoSWTPM(install_image):
    test_name = "no-tpm-no-swtpm"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.RemoveDevice("vtpm")

        # Verify we get expected error
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: no working TPM found, but install seed doesn't allow for use of swtpm")

def TestInstallSecureBootDisabledNoFallback(install_image):
    test_name = "secureboot-disabled-no-fallback"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)
    util._remove_secureboot_keys(test_image)

    with IncusTestVM(test_name, test_image) as vm:
        # Verify we get expected error
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: Secure Boot is disabled, but install seed doesn't allow this")
