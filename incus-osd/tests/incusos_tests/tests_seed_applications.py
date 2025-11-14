import os
import tempfile

from .incus_test_vm import IncusTestVM, util

def TestSeedApplictionsEmpty(install_image):
    test_name = "seed-applications-empty"
    test_seed = {
        "install.json": "{}",
        "applications.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System check error: at least one application must be defined in the provided applications seed")

def TestSeedApplictionsInvalid(install_image):
    test_name = "seed-applications-invalid"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"foobarbiz"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/sdb target=/dev/sda")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        vm.StopVM()
        vm.RemoveDevice("boot-media")

        # Start freshly installed IncusOS and verify successful boot.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "System is ready release="+incusos_version)

        # We shouldn't see anything about loading an application.
        vm.LogDoesntContain("incus-osd", "Downloading application")

def TestSeedApplictionsIncus(install_image):
    test_name = "seed-applications-incus"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="incus")

def TestSeedApplictionsIncusCeph(install_image):
    test_name = "seed-applications-incus-ceph"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus-ceph"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="incus-ceph")

        # We should also see Incus pulled in as a dependency
        vm.WaitExpectedLog("incus-osd", "Downloading application application=incus release="+incusos_version)

def TestSeedApplictionsIncusLinstor(install_image):
    test_name = "seed-applications-incus-linstor"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus-linstor"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="incus-linstor")

        # We should also see Incus pulled in as a dependency
        vm.WaitExpectedLog("incus-osd", "Downloading application application=incus release="+incusos_version)

def TestSeedApplictionsMigrationManager(install_image):
    test_name = "seed-applications-migration-manager"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"migration-manager"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="migration-manager")

def TestSeedApplictionsOperationsCenter(install_image):
    test_name = "seed-applications-operations-center"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"operations-center"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="operations-center")

def TestExternalSeedApplictionsMigrationManager(install_image):
    test_name = "external-seed-applications-migration-manager"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as seed_img:
        # Create and populate a user-provided ISO with seed files on it
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            with open(os.path.join(tmp_dir, "install.json"), "w") as seed:
                seed.write("{}")

            with open(os.path.join(tmp_dir, "applications.json"), "w") as seed:
                seed.write("""{"applications":[{"name":"migration-manager"}]}""")

            util._create_user_media(seed_img, tmp_dir, "iso", 0, "SEED_DATA")

        with IncusTestVM(test_name, test_image) as vm:
            vm.AttachISO(seed_img.name, "seed")

            vm.WaitSystemReady(incusos_version, application="migration-manager", remove_devices=["seed"])
