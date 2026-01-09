import os
import subprocess
import tempfile
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemReset(install_image):
    test_name = "incusos-api-system-reset"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Command the system to perform a factory reset.
        result = vm.APIRequest("/1.0/system/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        time.sleep(5)

        # Wait for the system to come back up.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestIncusOSAPISystemResetSWTPM(install_image):
    test_name = "incusos-api-system-reset-swtpm"
    test_seed = {
        "install.json": """{"security":{"missing_tpm":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Remove the tpm
        vm.RemoveDevice("vtpm")

        vm.WaitSystemReady(incusos_version)

        # Should see a log message about swtpm
        vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")

        # Command the system to perform a factory reset.
        result = vm.APIRequest("/1.0/system/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        time.sleep(5)

        # Wait for the system to reconfigure the swtpm.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Configuring swtpm-backed TPM on first boot, restarting in five seconds")
        vm.LogDoesntContain("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")

        time.sleep(5)

        # Wait for the system to come back up.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Should see a log message about swtpm
        vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")

def TestIncusOSAPISystemResetSWTPMToTPM(install_image):
    test_name = "incusos-api-system-reset-swtpm-to-tpm"
    test_seed = {
        "install.json": """{"security":{"missing_tpm":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        # Remove the tpm
        vm.RemoveDevice("vtpm")

        vm.WaitSystemReady(incusos_version)

        # Should see a log message about swtpm
        vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")

        # Command the system to perform a factory reset.
        result = vm.APIRequest("/1.0/system/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        time.sleep(5)

        # Stop the VM and attach a proper TPM
        vm.StopVM(force=True)
        vm.AddDevice("vtpm", "tpm")
        vm.StartVM()

        # Wait for the system to come back up.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestIncusOSAPISystemResetSecureBootDisabled(install_image):
    test_name = "incusos-api-system-reset-secureboot-disabled"
    test_seed = {
        "install.json": """{"security":{"missing_secure_boot":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)
    util._remove_secureboot_keys(test_image)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Should see a log message about SecureBoot being disabled
        vm.WaitExpectedLog("incus-osd", "Degraded security state: Secure Boot is disabled")

        # Command the system to perform a factory reset.
        result = vm.APIRequest("/1.0/system/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        time.sleep(5)

        # Wait for the system to come back up.
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Should see a log message about SecureBoot being disabled
        vm.WaitExpectedLog("incus-osd", "Degraded security state: Secure Boot is disabled")

def TestIncusOSAPISystemResetSecureBootDisabledToSB(install_image):
    test_name = "incusos-api-system-reset-secureboot-disabled-to-sb"
    test_seed = {
        "install.json": """{"security":{"missing_secure_boot":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
        util._extract_secureboot_keys(test_image, tmp_dir)
        util._remove_secureboot_keys(test_image)

        with IncusTestVM(test_name, test_image) as vm:
            vm.WaitSystemReady(incusos_version)

            # Should see a log message about SecureBoot being disabled
            vm.WaitExpectedLog("incus-osd", "Degraded security state: Secure Boot is disabled")

            # Copy the SecureBoot keys into the VM so we can enroll them to enable SecureBoot on next boot
            subprocess.run(["incus", "file", "push", tmp_dir+"/DB.auth", vm.vm_name+"/tmp/"], capture_output=True, check=True)
            subprocess.run(["incus", "file", "push", tmp_dir+"/KEK.auth", vm.vm_name+"/tmp/"], capture_output=True, check=True)
            subprocess.run(["incus", "file", "push", tmp_dir+"/PK.auth", vm.vm_name+"/tmp/"], capture_output=True, check=True)
            vm.RunCommand("efi-updatevar", "-f", "/tmp/DB.auth", "db")
            vm.RunCommand("efi-updatevar", "-f", "/tmp/KEK.auth", "KEK")
            vm.RunCommand("efi-updatevar", "-f", "/tmp/PK.auth", "PK")

            # Command the system to perform a factory reset.
            result = vm.APIRequest("/1.0/system/:factory-reset", method="POST")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            time.sleep(5)

            # Wait for the system to come back up.
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
            vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
            vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

            # Shouldn't see any mention of a degraded security state
            vm.LogDoesntContain("incus-osd", "Degraded security state:")
