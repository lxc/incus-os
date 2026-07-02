import os
import tempfile

from ..incus_test_vm import IncusTestVM, util

def TestExternalSeedApplictionsMigrationManager(install_image):
    test_name = "external-seed-applications-migration-manager"
    test_seed = None

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as seed_img:
        # Create and populate a user-provided ISO with seed files on it
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            with open(os.path.join(tmp_dir, "install.json"), "w") as seed:
                seed.write("{}")

            with open(os.path.join(tmp_dir, "applications.json"), "w") as seed:
                seed.write("""{"applications":[{"name":"migration-manager"}]}""")

            util._create_user_media(seed_img, tmp_dir, "iso", 0, "SEED_DATA")

        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AttachISO(seed_img.name, "seed")

            vm.WaitSystemReady(os_version, application="migration-manager", remove_devices=["seed"])

def TestExternalSeedInstallEmpty(install_image):
    test_name = "external-seed-install-empty"
    test_seed = None

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as seed_img:
        # Create and populate a user-provided ISO image with an empty install seed file on it
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            with open(os.path.join(tmp_dir, "install.yaml"), "w") as seed:
                seed.write("")

            util._create_user_media(seed_img, tmp_dir, "iso", 0, "SEED_DATA")

        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AttachISO(seed_img.name, "seed")

            # Perform IncusOS install.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
            vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestExternalSeedInstallTarget(install_image):
    test_name = "external-seed-install-target"
    test_seed = None

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as seed_img:
            # Create and populate a user-provided USB stick with a seed file on it
            with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
                with open(os.path.join(tmp_dir, "install.json"), "w") as seed:
                    seed.write("""{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""")

                util._create_user_media(seed_img, tmp_dir, "img", 1024*1024*1024, "SEED_DATA")

            with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
                vm.AddDevice("disk1", "disk", "source="+disk_img.name)
                vm.AddDevice("recovery", "disk", "source="+seed_img.name, "io.bus=usb")

                # Perform IncusOS install.
                vm.StartVM()
                vm.WaitAgentRunning()
                vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
                vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")
