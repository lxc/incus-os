import os
import subprocess
import tempfile

from . import IncusTestVM, util

def TestRecoveryHotfixUSB(install_image):
    test_name = "recovery-hotfix-usb"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)
        vm.StopVM()

        # Run the hotfix script from a USB stick.
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
            # Create a vfat partition labeled RESCUE_DATA and copy the hotfix script.
            recovery_img.truncate(1024*1024*1024)
            subprocess.run(["/sbin/sgdisk", "-n", "1", "-c", "1:RESCUE_DATA", recovery_img.name], capture_output=True, check=True)
            subprocess.run(["/sbin/mkfs.vfat", "-S", "512", "--offset=2048", recovery_img.name], capture_output=True, check=True)
            subprocess.run(["mcopy", "-i", recovery_img.name+"@@1048576", "hotfix.sh.sig", "::/hotfix.sh.sig"], capture_output=True, check=True)

            # Attach the recovery media.
            vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

            # Test the hotfix script.
            _hotfix_common(vm)

            # Remove the recovery media.
            vm.RemoveDevice("recovery")

def TestRecoveryHotfixISO(install_image):
    test_name = "recovery-hotfix-iso"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)
        vm.StopVM()

        # Run the hotfix from an ISO image.
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_iso:
            # Create an ISO image labeled RESCUE_DATA and copy the hotfix script.
            subprocess.run(["mkisofs", "-V", "RESCUE_DATA", "-joliet-long", "-rock", "-o", recovery_iso.name, "hotfix.sh.sig"], capture_output=True, check=True)

            # Attach the recovery media.
            subprocess.run(["incus", "storage", "volume", "import", "default", recovery_iso.name, os.path.basename(recovery_iso.name), "--type=iso"], capture_output=True, check=True)
            vm.AddDevice("recovery", "disk", "pool=default", "source="+os.path.basename(recovery_iso.name))

            try:
                # Test the hotfix script.
                _hotfix_common(vm)

            finally:
                # Remove the recovery media.
                vm.RemoveDevice("recovery")
                subprocess.run(["incus", "storage", "volume", "delete", "default", os.path.basename(recovery_iso.name)], capture_output=True, check=True)

def _hotfix_common(vm):
    vm.StartVM()
    vm.WaitAgentRunning()

    vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
    vm.WaitExpectedLog("incus-osd", "Hotfix script detected, verifying signature")
    vm.WaitExpectedLog("incus-osd", "Running hotfix script")
    vm.WaitExpectedLog("incus-osd", "Recovery actions completed")

    vm.RunCommand("stat", "/tmp/hotfix.status")

def TestRecoveryUpdateUSB(install_image):
    test_name = "recovery-update-usb"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)
        vm.StopVM()

        # Run the update script from a USB stick.
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
            # Create a vfat partition labeled RESCUE_DATA and copy the updates.
            recovery_img.truncate(4*1024*1024*1024)
            subprocess.run(["/sbin/sgdisk", "-n", "1", "-c", "1:RESCUE_DATA", recovery_img.name], capture_output=True, check=True)
            subprocess.run(["/sbin/mkfs.vfat", "-S", "512", "--offset=2048", recovery_img.name], capture_output=True, check=True)
            subprocess.run(["mcopy", "-i", recovery_img.name+"@@1048576", "updates/", "::/updates/"], capture_output=True, check=True)

            # Attach the recovery media.
            vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

            # Test the updates.
            _update_common(vm)

            # Remove the recovery media.
            vm.RemoveDevice("recovery")

            vm.StopVM()
            vm.StartVM()

            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Starting application name=incus version=999901010000")
            vm.WaitExpectedLog("incus-osd", "System is ready release=999901010000")

def TestRecoveryUpdateISO(install_image):
    test_name = "recovery-update-iso"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)
        vm.StopVM()

        # Run the update from an ISO image.
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_iso:
            # Create an ISO image labeled RESCUE_DATA and copy the updates.
            subprocess.run(["mkisofs", "-V", "RESCUE_DATA", "-joliet-long", "-rock", "-o", recovery_iso.name, "updates/"], capture_output=True, check=True)

            # Attach the recovery media.
            subprocess.run(["incus", "storage", "volume", "import", "default", recovery_iso.name, os.path.basename(recovery_iso.name), "--type=iso"], capture_output=True, check=True)
            vm.AddDevice("recovery", "disk", "pool=default", "source="+os.path.basename(recovery_iso.name))

            try:
                # Test the updates.
                _update_common(vm)

            finally:
                # Remove the recovery media.
                vm.RemoveDevice("recovery")
                subprocess.run(["incus", "storage", "volume", "delete", "default", os.path.basename(recovery_iso.name)], capture_output=True, check=True)

            vm.StopVM()
            vm.StartVM()

            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Starting application name=incus version=999901010000")
            vm.WaitExpectedLog("incus-osd", "System is ready release=999901010000")

def _update_common(vm):
    vm.StartVM()
    vm.WaitAgentRunning()

    vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
    vm.WaitExpectedLog("incus-osd", "Update metadata detected, verifying signature")
    vm.WaitExpectedLog("incus-osd", "Decompressing and verifying each update file")
    vm.WaitExpectedLog("incus-osd", "Applying application update(s)")
    vm.WaitExpectedLog("incus-osd", "Applying OS update(s)")
    vm.WaitExpectedLog("incus-osd", "Recovery actions completed")
