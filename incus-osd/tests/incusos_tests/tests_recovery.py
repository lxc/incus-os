import json
import os
import tempfile
import urllib.request

from .incus_test_vm import IncusTestVM, util

def TestRecoveryUpdateFromUSB(install_image):
    test_name = "recovery-update-from-usb"
    test_seed = {
        "install.json": "{}",
        # Purposefully don't configure any addresses on the interface to prevent automatic downloading of the incus application.
        "network.json": """{"interfaces":[{"name":"enp5s0","hwaddr":"enp5s0","required_for_online":"no"}]}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        _installStartChecks(vm, incusos_version)

        # Apply the updates from a USB stick.
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            _fetchUpdateFiles(tmp_dir, incusos_version)

            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
                # Create a vfat partition labeled RESCUE_DATA and copy the updates.
                util._create_user_media(recovery_img, tmp_dir, "img", 4*1024*1024*1024, "RESCUE_DATA")

                # Stop the VM and attach the recovery media.
                vm.StopVM()
                vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

                _recoveryChecks(vm, incusos_version)

def TestRecoveryUpdateFromISO(install_image):
    test_name = "recovery-update-from-iso"
    test_seed = {
        "install.json": "{}",
        # Purposefully don't configure any addresses on the interface to prevent automatic downloading of the incus application.
        "network.json": """{"interfaces":[{"name":"enp5s0","hwaddr":"enp5s0","required_for_online":"no"}]}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        _installStartChecks(vm, incusos_version)

        # Apply the updates from an ISO image.
        with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
            _fetchUpdateFiles(tmp_dir, incusos_version)

            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_iso:
                # Create an ISO labeled RESCUE_DATA containing the updates.
                util._create_user_media(recovery_iso, tmp_dir, "iso", 0, "RESCUE_DATA")

                # Stop the VM and attach the recovery media.
                vm.StopVM()
                vm.AttachISO(recovery_iso.name, "recovery")

                _recoveryChecks(vm, incusos_version)

def _fetchUpdateFiles(directory, incusos_version):
    os.mkdir(directory+"/update")

    urllib.request.urlretrieve("https://images.linuxcontainers.org/os/"+incusos_version+"/update.json", directory+"/update/update.json")
    urllib.request.urlretrieve("https://images.linuxcontainers.org/os/"+incusos_version+"/update.sjson", directory+"/update/update.sjson")

    os.mkdir(directory+"/update/x86_64")

    with open(directory+"/update/update.json") as f:
        j = json.load(f)

        for updateFile in j["files"]:
            if updateFile["architecture"] != "x86_64":
                continue

            if updateFile["type"] == "image-raw" or updateFile["type"] == "image-iso" or updateFile["type"] == "image-manifest" or updateFile["type"] == "changelog":
                continue

            if updateFile["type"] == "application" and updateFile["component"] != "incus":
                continue

            urllib.request.urlretrieve("https://images.linuxcontainers.org/os/"+incusos_version+"/"+updateFile["filename"], directory+"/update/"+updateFile["filename"])

def _installStartChecks(vm, incusos_version):
    # Perform IncusOS install.
    vm.StartVM()
    vm.WaitAgentRunning()
    vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/sdb target=/dev/sda")
    vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

    # Stop the VM post-install and remove install media.
    vm.StopVM()
    vm.RemoveDevice("boot-media")

    # Start the VM; we expect network configuration to fail since we don't specify any actual addresses.
    vm.StartVM()
    vm.WaitAgentRunning()
    vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
    vm.WaitExpectedLog("incus-osd", "Bringing up the network")
    vm.WaitExpectedLog("incus-osd", "systemd-timesyncd failed to perform NTP synchronization, system time may be incorrect")
    vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

    # Verify that no applications are installed.
    result = vm.APIRequest("/1.0/applications")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

    if len(result["metadata"]) != 0:
        raise IncusOSException("expected no application to be installed")

def _recoveryChecks(vm, incusos_version):
    # Start the VM and install incus from the recovery media.
    vm.StartVM()
    vm.WaitAgentRunning()
    vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
    vm.WaitExpectedLog("incus-osd", "Update metadata detected, verifying signature")
    vm.WaitExpectedLog("incus-osd", "Decompressing and verifying each update file")
    vm.WaitExpectedLog("incus-osd", "Applying application update(s)")
    vm.WaitExpectedLog("incus-osd", "Applying OS update(s)")
    vm.WaitExpectedLog("incus-osd", "Recovery actions completed")
    vm.WaitExpectedLog("incus-osd", "Bringing up the network")
    vm.WaitExpectedLog("incus-osd", "systemd-timesyncd failed to perform NTP synchronization, system time may be incorrect")
    vm.WaitExpectedLog("incus-osd", "Starting application name=incus version="+incusos_version)
    vm.WaitExpectedLog("incus-osd", "Initializing application name=incus version="+incusos_version)
    vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

    # Verity the incus application is installed.
    result = vm.APIRequest("/1.0/applications")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

    if len(result["metadata"]) != 1:
        raise IncusOSException("expected exactly one application")

    if result["metadata"][0] != "/1.0/applications/incus":
        raise IncusOSException("expected the incus application to be installed")
