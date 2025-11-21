import os
import subprocess
import tempfile

from .incus_test_vm import IncusTestVM, util

def TestInstallNoTPM(install_image):
    test_name = "no-tpm"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.RemoveDevice("vtpm")

        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: no working TPM device found")

def TestInstallNoSeed(install_image):
    test_name = "no-seed"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: unable to begin install without seed configuration")

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
