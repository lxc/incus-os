import os
import subprocess
import tempfile
import time

from ..incus_test_vm import IncusTestVM, util

def TestSeedInstallReboot(install_image):
    test_name = "seed-install-reboot"
    test_seed = {
        "install.json": """{"force_reboot":true}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
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

def TestSeedInstallTargetID(install_image):
    test_name = "seed-install-target-id"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestSeedInstallTargetBus(install_image):
    test_name = "seed-install-target-bus"
    test_seed = {
        "install.json": """{"target":{"bus":"nvme","sort_order":"smallest"}}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(55*1024*1024*1024)

        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name, "io.bus=nvme")

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk1")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestSeedInstallForce(install_image):
    test_name = "seed-install-force"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_disk1"},"force_install":true}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        # Truncate the disk image file to 50GiB and setup a single GPT partition.
        # The presence of the existing GPT table will cause install to fail unless
        # "ForceInstall" is set to true.
        disk_img.truncate(50*1024*1024*1024)
        subprocess.run(["/sbin/sgdisk", "-n", "1", disk_img.name], capture_output=True, check=True)

        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestSeedInstallForceConfirmation(install_image):
    test_name = "seed-install-force-confirmation"
    test_seed = {
        "install.yaml": "",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Wait for a normal install to complete
        vm.WaitSystemReady(os_version)

        # Get the machine ID
        output = vm.RunCommand("cat", "/etc/machine-id")
        confirmationValue = output.stdout.decode("utf-8")[:6]

        # Re-attach the install media and update the install seed
        vm.AddDevice("boot-media", "disk", "source="+test_image, "io.bus=usb", "boot.priority=10", "readonly=false")

        # Allow time for the device to appear
        time.sleep(5)

        vm.RunCommand("mkdir", "/tmp/seed/")
        vm.RunCommand("tar", "xf", "/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0-part2", "-C", "/tmp/seed/")
        vm.RunCommand("sh", "-c", "echo 'force_install: true' >> /tmp/seed/install.yaml")
        vm.RunCommand("tar", "cf", "/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0-part2", "-C", "/tmp/seed/", ".")

        # Restart the VM and check that we get an error about ForceInstallConfirmation.
        vm.StopVM()
        vm.StartVM()
        time.sleep(5)
        vm.WaitAgentRunning()

        vm.WaitExpectedLog("incus-osd", os_name + " is already installed on this system; to confirm overwriting, add `ForceInstallConfirmation=" + confirmationValue + "` to the install seed and reboot")

        vm.RunCommand("mkdir", "/tmp/seed/")
        vm.RunCommand("tar", "xf", "/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0-part2", "-C", "/tmp/seed/")
        vm.RunCommand("sh", "-c", "echo 'force_install_confirmation: " + confirmationValue + "' >> /tmp/seed/install.yaml")
        vm.RunCommand("tar", "cf", "/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0-part2", "-C", "/tmp/seed/", ".")

        # Restart the VM and check that we then properly wipe the existing install.
        vm.StopVM()
        vm.StartVM()
        time.sleep(5)
        vm.WaitAgentRunning()

        vm.WaitExpectedLog("incus-osd", "Wiping existing version of " + os_name + ", then rebooting in five seconds to run actual installation")

        # Wait for the VM to auto-reboot and begin the installation once again
        time.sleep(10)
        vm.WaitAgentRunning()

        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=")

def TestSeedInstallEmpty(install_image):
    test_name = "seed-install-empty"
    test_seed = {
        "install.json": ""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")
