import os
import tempfile
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

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
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and expect an immediate upgrade.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update")
        match = vm.WaitExpectedLog("incus-osd", "Downloading OS update version=(\\d+)", regex=True)
        new_version = match.group(1)
        vm.WaitExpectedLog("incus-osd", "Applying OS update version="+new_version)

        # Allow some time for the update to apply.
        time.sleep(30)

        # Wait for the system to automatically reboot after installing the upgrade.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready version="+new_version)

def TestBaselineUpgradeOSOnly(install_image):
    test_name = "baseline-upgrade-os-only"
    test_seed = {
        "install.json": "{}",
        "provider.json": """{"name":"local"}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS; shouldn't see any attempts at applying an update.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        vm.LogDoesntContain("incus-osd", "Downloading OS update")
        vm.LogDoesntContain("incus-osd", "Downloading application update")

        # Now that we've started up, switch the provider back to the main "images" and check for an OS update.
        result = vm.APIRequest("/1.0/system/provider", method="PUT", body="""{"config":{"name":"images"}}""")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        result = vm.APIRequest("/1.0/system/update/:check", method="POST", body="""{"os_only":true}""")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        match = vm.WaitExpectedLog("incus-osd", "Downloading OS update version=(\\d+)", regex=True)
        new_version = match.group(1)
        vm.WaitExpectedLog("incus-osd", "Applying OS update version="+new_version)

        if new_version == incusos_version:
            raise IncusOSException("expected a different OS version when applying update")

        vm.LogDoesntContain("incus-osd", "Downloading application update")

def TestBaselineUpgradeApplicationOnly(install_image):
    test_name = "baseline-upgrade-application-only"
    test_seed = {
        "install.json": "{}",
        "provider.json": """{"name":"local"}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Prepare to install an older version of the incus app via the recovery mechanism.
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            util._manual_download_application(tmp_dir, "incus", incusos_version)

            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
                # Create a vfat partition labeled RESCUE_DATA and copy the updates.
                util._create_user_media(recovery_img, tmp_dir, "img", 4*1024*1024*1024, "RESCUE_DATA")

                vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

                vm.StartVM()
                vm.WaitAgentRunning()
                vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
                vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
                vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
                vm.WaitExpectedLog("incus-osd", "Update metadata detected, verifying signature")
                vm.WaitExpectedLog("incus-osd", "Processing validated update metadata version="+incusos_version)
                vm.WaitExpectedLog("incus-osd", "Decompressing and verifying each update file")
                vm.WaitExpectedLog("incus-osd", "Skipping missing file: 'x86_64/debug.raw.gz")
                vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
                vm.WaitExpectedLog("incus-osd", "Recovery actions completed")
                vm.WaitExpectedLog("incus-osd", "Bringing up the network")
                vm.WaitExpectedLog("incus-osd", "Starting application name=incus version="+incusos_version)
                vm.WaitExpectedLog("incus-osd", "Initializing application name=incus version="+incusos_version)
                vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

                vm.LogDoesntContain("incus-osd", "Downloading OS update")

                # Now that we've started up, switch the provider back to the main "images" and check for an application update.
                result = vm.APIRequest("/1.0/system/provider", method="PUT", body="""{"config":{"name":"images"}}""")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                result = vm.APIRequest("/1.0/applications/incus/:check-update", method="POST")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                match = vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version=((?!" + incusos_version + ")\\d+)", regex=True)
                new_version = match.group(1)
                vm.WaitExpectedLog("incus-osd", "Reloading application name=incus version="+new_version)

                if new_version == incusos_version:
                    raise IncusOSException("expected a different application version when applying update")

                vm.LogDoesntContain("incus-osd", "Downloading OS update")
