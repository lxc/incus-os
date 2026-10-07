import os
import tempfile
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestBaselineUpgrade(install_image):
    test_name = "baseline-upgrade"
    test_seed = {
        "install.json": "{}"
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

        # Start freshly installed IncusOS and expect an immediate upgrade.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update")
        match = vm.WaitExpectedLog("incus-osd", "Downloading OS update channel=stable version=(\\d+)", regex=True)
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
        "provider.json": """{"name":"debug"}"""
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

        # Start freshly installed IncusOS; shouldn't see any attempts at applying an update.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "System is ready version="+os_version)

        vm.LogDoesntContain("incus-osd", "Downloading OS update")
        vm.LogDoesntContain("incus-osd", "Downloading application update")

        IMAGES_SERVER = os.getenv("IMAGES_SERVER", "https://images.linuxcontainers.org")

        # Now that we've started up, switch the provider back to the main "images" and check for an OS update.
        result = vm.APIRequest("/1.0/system/provider", method="PUT", body='{"config":{"name":"images","config":{"server_url":"' + IMAGES_SERVER + '/os"}}}', use_unix_socket=True)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        result = vm.APIRequest("/1.0/system/update/:check", method="POST", body="""{"os_only":true}""", use_unix_socket=True)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        match = vm.WaitExpectedLog("incus-osd", "Downloading OS update channel=stable version=(\\d+)", regex=True)
        new_version = match.group(1)
        vm.WaitExpectedLog("incus-osd", "Applying OS update version="+new_version)

        if new_version == os_version:
            raise IncusOSException("expected a different OS version when applying update")

        vm.LogDoesntContain("incus-osd", "Downloading application update")

def TestBaselineUpgradeApplicationOnly(install_image):
    test_name = "baseline-upgrade-application-only"
    test_seed = {
        "install.json": "{}",
        "provider.json": """{"name":"debug"}"""
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

        # Prepare to install an older version of the incus app via the recovery mechanism.
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            util._manual_download_application(tmp_dir, ["incus"], os_version)

            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
                # Create a vfat partition labeled RESCUE_DATA and copy the updates.
                util._create_user_media(recovery_img, tmp_dir, "img", 4*1024*1024*1024, "RESCUE_DATA")

                vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

                vm.StartVM()
                vm.WaitAgentRunning()
                vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
                vm.WaitExpectedLog("incus-osd", "Update metadata detected, verifying signature")
                vm.WaitExpectedLog("incus-osd", "Processing validated update metadata version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Decompressing and verifying each update file")
                vm.WaitExpectedLog("incus-osd", "Skipping missing file: 'x86_64/debug.raw.gz")
                vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus channel=stable version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Recovery actions completed")
                vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
                vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
                vm.WaitExpectedLog("incus-osd", "Bringing up the network")
                vm.WaitExpectedLog("incus-osd", "Starting application name=incus version=.+ \\["+os_version+"\\]", regex=True)
                vm.WaitExpectedLog("incus-osd", "Initializing application name=incus version=.+ \\["+os_version+"\\]", regex=True)
                vm.WaitExpectedLog("incus-osd", "System is ready version="+os_version)

                vm.LogDoesntContain("incus-osd", "Downloading OS update")

                IMAGES_SERVER = os.getenv("IMAGES_SERVER", "https://images.linuxcontainers.org")

                # Now that we've started up, switch the provider back to the main "images" and check for an application update.
                result = vm.APIRequest("/1.0/system/provider", method="PUT", body='{"config":{"name":"images","config":{"server_url":"' + IMAGES_SERVER + '/os"}}}')
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                result = vm.APIRequest("/1.0/applications/incus/:check-update", method="POST")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                match = vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus channel=stable version=((?!" + os_version + ")\\d+)", regex=True)
                new_version = match.group(1)
                vm.WaitExpectedLog("incus-osd", "Reloading application name=incus version="+new_version)

                if new_version == os_version:
                    raise IncusOSException("expected a different application version when applying update")

                vm.LogDoesntContain("incus-osd", "Downloading OS update")

def TestBaselineUpgradeApplicationPinning(install_image):
    test_name = "baseline-upgrade-application-pinning"
    test_seed = {
        "install.json": "{}",
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus"},{"name":"incus-ceph"},{"name":"incus-linstor"},{"name":"openfga"}]}""",
        "provider.json": """{"name":"debug"}"""
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

        # Prepare to install an older versions of several applications via the recovery mechanism.
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            util._manual_download_application(tmp_dir, ["incus", "incus-ceph", "incus-linstor", "openfga"], os_version)

            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
                # Create a vfat partition labeled RESCUE_DATA and copy the updates.
                util._create_user_media(recovery_img, tmp_dir, "img", 4*1024*1024*1024, "RESCUE_DATA")

                vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

                vm.StartVM()
                vm.WaitAgentRunning()
                vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
                vm.WaitExpectedLog("incus-osd", "Update metadata detected, verifying signature")
                vm.WaitExpectedLog("incus-osd", "Processing validated update metadata version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Decompressing and verifying each update file")
                vm.WaitExpectedLog("incus-osd", "Skipping missing file: 'x86_64/debug.raw.gz")
                vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus channel=stable version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus-ceph channel=stable version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus-linstor channel=stable version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Downloading application update application=openfga channel=stable version="+os_version)
                vm.WaitExpectedLog("incus-osd", "Recovery actions completed")
                vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
                vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
                vm.WaitExpectedLog("incus-osd", "Bringing up the network")
                vm.WaitExpectedLog("incus-osd", "Starting application name=incus version=.+ \\["+os_version+"\\]", regex=True)
                vm.WaitExpectedLog("incus-osd", "Initializing application name=incus version=.+ \\["+os_version+"\\]", regex=True)
                vm.WaitExpectedLog("incus-osd", "System is ready version="+os_version)

                vm.LogDoesntContain("incus-osd", "Downloading OS update")

                # Pin the versions of incus, incus-ceph and openfga.
                result = vm.APIRequest("/1.0/applications/incus", method="PUT", body='{"config":{"pin_current_versions":true}}')
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                # Sleep a few seconds to allow the incus service to restart.
                time.sleep(5)

                result = vm.APIRequest("/1.0/applications/incus-ceph", method="PUT", body='{"config":{"pin_current_versions":true}}')
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                result = vm.APIRequest("/1.0/applications/openfga", method="PUT", body='{"config":{"pin_current_versions":true,"api_tokens":["foobarbiz"]}}')
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                IMAGES_SERVER = os.getenv("IMAGES_SERVER", "https://images.linuxcontainers.org")

                # Now that we've started up, switch the provider back to the main "images" and check for system updates.
                result = vm.APIRequest("/1.0/system/provider", method="PUT", body='{"config":{"name":"images","config":{"server_url":"' + IMAGES_SERVER + '/os"}}}')
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                result = vm.APIRequest("/1.0/system/update/:check", method="POST")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                # Should only see an update applied for the incus-linstor application.
                match = vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus-linstor channel=stable version=((?!" + os_version + ")\\d+)", regex=True)
                new_version = match.group(1)
                vm.WaitExpectedLog("incus-osd", "Starting application name=incus-linstor version=\\S+ \\["+new_version+"\\]", regex=True)

                if new_version == os_version:
                    raise IncusOSException("expected a different application version when applying update")

                vm.WaitExpectedLog("incus-osd", "Skipping application update because version pinning is enabled app=incus")
                vm.WaitExpectedLog("incus-osd", "Skipping application update because version pinning is enabled app=incus-ceph")
                vm.WaitExpectedLog("incus-osd", "Skipping application update because version pinning is enabled app=openfga")

                vm.WaitExpectedLog("incus-osd", "Downloading OS update channel=stable version="+new_version)
                vm.WaitExpectedLog("incus-osd", "Applying OS update version="+new_version)

                # Should only have a single version of the incus, incus-ceph, and openfga applications.
                result = vm.APIRequest("/1.0/applications/incus")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                if len(result["metadata"]["state"]["available_versions"]) != 1:
                    raise IncusOSException("expected exactly one version of the incus application")

                result = vm.APIRequest("/1.0/applications/incus-ceph")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                if len(result["metadata"]["state"]["available_versions"]) != 1:
                    raise IncusOSException("expected exactly one version of the incus-ceph application")

                result = vm.APIRequest("/1.0/applications/openfga")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                if len(result["metadata"]["state"]["available_versions"]) != 1:
                    raise IncusOSException("expected exactly one version of the openfga application")

                # Remove pin for incus-ceph, and then expect to see an update applied.
                result = vm.APIRequest("/1.0/applications/incus-ceph", method="PUT", body='{"config":{"pin_current_versions":false}}')
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                result = vm.APIRequest("/1.0/system/update/:check", method="POST")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus-ceph channel=stable version="+new_version)
                vm.WaitExpectedLog("incus-osd", "Starting application name=incus-ceph version=\\S+ \\["+new_version+"\\]", regex=True)

                result = vm.APIRequest("/1.0/applications/incus-ceph")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

                if len(result["metadata"]["state"]["available_versions"]) != 2:
                    raise IncusOSException("expected exactly two versions of the incus-ceph application")
