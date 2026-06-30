from ..incus_test_vm import IncusTestVM, util

def TestSeedApplictionsEmpty(install_image):
    test_name = "seed-applications-empty"
    test_seed = {
        "install.json": "{}",
        "applications.json": "{}"
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
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

        # Start freshly installed IncusOS and verify error about an invalid application.
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Upgrading LUKS TPM PCR bindings, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "ERROR failed to check dependencies for application 'foobarbiz': unknown application")
        vm.WaitExpectedLog("incus-osd", "System is ready version="+os_version)

        # We shouldn't see anything about loading an invalid application.
        vm.LogDoesntContain("incus-osd", "Downloading application update application=foobarbiz")

def TestSeedApplictionsDebug(install_image):
    test_name = "seed-applications-debug"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"debug"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="debug")

def TestSeedApplictionsGPUSupport(install_image):
    test_name = "seed-applications-gpu-support"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"gpu-support"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="gpu-support")

def TestSeedApplictionsIncus(install_image):
    test_name = "seed-applications-incus"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="incus")

def TestSeedApplictionsIncusLTS70(install_image):
    test_name = "seed-applications-incus-lts-70"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus-lts-7.0"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="incus-lts-7.0")

def TestSeedApplictionsIncusCeph(install_image):
    test_name = "seed-applications-incus-ceph"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus-ceph"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="incus-ceph")

        # We should also see Incus pulled in as a dependency
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus channel=stable version="+os_version)

def TestSeedApplictionsIncusLinstor(install_image):
    test_name = "seed-applications-incus-linstor"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"incus-linstor"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="incus-linstor")

        # We should also see Incus pulled in as a dependency
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus channel=stable version="+os_version)

def TestSeedApplictionsMigrationManager(install_image):
    test_name = "seed-applications-migration-manager"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"migration-manager"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="migration-manager")

def TestSeedApplictionsOpenFGA(install_image):
    test_name = "seed-applications-openfga"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"openfga"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="openfga")

def TestSeedApplictionsOperationsCenter(install_image):
    test_name = "seed-applications-operations-center"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"operations-center"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, application="operations-center")
