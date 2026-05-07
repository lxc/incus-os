import os
import subprocess
import tempfile
import time

from .incus_test_vm import IncusTestVM, util

def TestSeedInstallReboot(install_image):
    test_name = "seed-install-reboot"
    test_seed = {
        "install.json": """{"force_reboot":true}"""
    }

    test_image, os_name, os_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

        # Wait for the VM to auto-reboot.
        time.sleep(15)

        # Since we don't remove the install media, expect an error which is fine for this test.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: install media detected, but the system is already installed; please remove USB/CDROM and reboot the system")

def TestSeedInstallTarget(install_image):
    test_name = "seed-install-target"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}"""
    }

    test_image, os_name, os_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        with IncusTestVM(os_name, test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestSeedInstallForce(install_image):
    test_name = "seed-install-force"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_disk1"},"force_install":true}"""
    }

    test_image, os_name, os_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        # Truncate the disk image file to 50GiB and setup a single GPT partition.
        # The presence of the existing GPT table will cause install to fail unless
        # "ForceInstall" is set to true.
        disk_img.truncate(50*1024*1024*1024)
        subprocess.run(["/sbin/sgdisk", "-n", "1", disk_img.name], capture_output=True, check=True)

        with IncusTestVM(os_name, test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestSeedInstallEmpty(install_image):
    test_name = "seed-install-empty"
    test_seed = {
        "install.json": ""
    }

    test_image, os_name, os_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestExternalSeedInstallEmpty(install_image):
    test_name = "external-seed-install-empty"
    test_seed = None

    test_image, os_name, os_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as seed_img:
        # Create and populate a user-provided ISO image with an empty install seed file on it
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            with open(os.path.join(tmp_dir, "install.yaml"), "w") as seed:
                seed.write("")

            util._create_user_media(seed_img, tmp_dir, "iso", 0, "SEED_DATA")

        with IncusTestVM(os_name, test_name, test_image) as vm:
            vm.AttachISO(seed_img.name, "seed")

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestExternalSeedInstallTarget(install_image):
    test_name = "external-seed-install-target"
    test_seed = None

    test_image, os_name, os_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as seed_img:
            # Create and populate a user-provided USB stick with a seed file on it
            with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
                with open(os.path.join(tmp_dir, "install.json"), "w") as seed:
                    seed.write("""{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""")

                util._create_user_media(seed_img, tmp_dir, "img", 1024*1024*1024, "SEED_DATA")

            with IncusTestVM(os_name, test_name, test_image) as vm:
                vm.AddDevice("disk1", "disk", "source="+disk_img.name)
                vm.AddDevice("recovery", "disk", "source="+seed_img.name, "io.bus=usb")

                # Perform IncusOS install.
                vm.StartVM()
                vm.WaitAgentRunning()
                vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
                vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")
