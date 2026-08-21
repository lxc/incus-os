import os
import re
import subprocess
import tempfile
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestSecureBootKeyRotation(install_image):
    # Create the test VM
    test_name = "test-secureboot-key-rotation"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        testSecureBootKeyRotation(vm, os_name, os_version)

def TestSecureBootKeyRotationSWTPM(install_image):
    # Create the test VM
    test_name = "test-secureboot-key-rotation-swtpm"
    test_seed = {
        "install.json": """{"security":{"missing_tpm":true}}""",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Remove the tpm
        vm.RemoveDevice("vtpm")

        vm.WaitSystemReady(os_version)

        # Should see a log message about swtpm
        vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")

        testSecureBootKeyRotation(vm, os_name, os_version)

def TestUpdateMetadata(install_image):
    # Create the test VM
    test_name = "test-update-metadata"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        testUpdateMetadata(vm, os_version)

def TestHotfixScript(install_image):
    # Create the test VM
    test_name = "test-hotfix-script"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        testHotfixScriptAPI(vm)

        testHotfixScriptRecovery(vm)

def testSecureBootKeyRotation(vm, os_name, os_version):
    """Test rotation of a SecureBoot key that is used to sign UKIs."""

    # First, verify our "complex" dbx SecureBoot variable is properly configured.
    # It consists of an expired Microsoft certificate, certificate hashes of another
    # Microsoft certificate, and the SHA256 of a random IncusOS UKI. This should
    # cover pretty much any type of entry that could exist in the dbx variable.
    output = vm.RunCommand("efi-readvar")
    if "dbx: List 0, type X509" not in output.stdout.decode("utf-8"):
        raise IncusOSException("expected x509 certificate not present in dbx")

    if "dbx: List 3, type Unknown" not in output.stdout.decode("utf-8"):
        raise IncusOSException("expected certificate hash not present in dbx")

    if "dbx: List 4, type SHA256" not in output.stdout.decode("utf-8"):
        raise IncusOSException("expected PE binary hash not present in dbx")

    # Second, get the list of certificates from the IncusOS API.
    result = vm.APIRequest("/1.0/system/security")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    certs = result["metadata"]["state"]["secure_boot_certificates"]
    if len(certs) != 6:
        raise IncusOSException("expected six SecureBoot certificates to be present")

    if certs[3]["subject"] != "CN=TestOS - Secure Boot 1 R1,O=TestOS":
        raise IncusOSException("expected fourth SecureBoot certificate to be 'Secure Boot 1'")

    if certs[4]["subject"] != "CN=TestOS - Secure Boot 2 R1,O=TestOS":
        raise IncusOSException("expected fifth SecureBoot certificate to be 'Secure Boot 2'")

    if certs[5]["subject"] != "CN=Microsoft Corporation UEFI CA 2011,O=Microsoft Corporation,L=Redmond,ST=Washington,C=US":
        raise IncusOSException("expected sixth SecureBoot certificate to a Microsoft CA")

    # Apply the first db and dbx updates, which will each trigger a VM restart.
    with open("sb-update.tar", "rb") as update:
        update_bytes = update.read()

        # Apply the db update.
        result = vm.APIRequest("/1.0/debug/secureboot/:update", method="POST", body=update_bytes, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        vm.WaitExpectedLog("incus-osd", "Appending certificate SHA256:[0-9A-F]{64} to EFI variable db", regex=True)

        time.sleep(5)
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready")

        # Apply the dbx update.
        result = vm.APIRequest("/1.0/debug/secureboot/:update", method="POST", body=update_bytes, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        vm.WaitExpectedLog("incus-osd", "Appending certificate SHA256:[0-9A-F]{64} to EFI variable dbx", regex=True)

        time.sleep(5)
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready")

        # Expect to get back eight SecureBoot certificates from the API.
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        certs = result["metadata"]["state"]["secure_boot_certificates"]
        if len(certs) != 8:
            raise IncusOSException("expected eight SecureBoot certificates to be present")

    # Try to apply a dbx update for the certificate that's signed the running UKI. This should
    # fail, since actually applying the update would brick the ability to boot the running UKI.
    with open("sb-uki-revoke.tar", "rb") as update:
        update_bytes = update.read()

        # Apply the dbx update.
        result = vm.APIRequest("/1.0/debug/secureboot/:update", method="POST", body=update_bytes, content_type="application/x-tar")
        if result["status_code"] == 200:
            raise IncusOSException("unexpected success applying dbx update")

        if not re.search("unable to apply dbx update, since UKI image '/boot/EFI/Linux/IncusOS_[0-9]+.efi' is signed by the key which would be revoked", result["error"]):
            raise IncusOSException("got unexpected error applying dbx update: " + result["error"])

    # Trigger an update to the second build, which is signed by a different SecureBoot key.
    result = vm.APIRequest("/1.0/system/update", method="PUT", body="""{"config":{"auto_reboot":false,"channel":"testing","check_frequency":"6h"}}""")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    result = vm.APIRequest("/1.0/system/update/:check", method="POST")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    vm.WaitExpectedLog("incus-osd", "Reloading application name=incus")

    time.sleep(10)
    result = vm.APIRequest("/1.0/system/:reboot", method="POST")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))
    time.sleep(10)

    vm.WaitAgentRunning()
    vm.WaitExpectedLog("incus-osd", "System is ready")

    # After rebooting into the new UKI, manually remove the older UKI that's still
    # present under /boot/. This is because the dbx update check will verify if any
    # existing UKI would be rendered unbootable. Normally after two updates the old
    # UKI would be automatically removed, but for testing remove it by hand so we only
    # need one round of updates.
    vm.RunCommand("rm", "/boot/EFI/Linux/"+os_name+"_"+os_version+".efi")

    # Now, apply the dbx update revoking the certificate that was used by the older UKI.
    with open("sb-uki-revoke.tar", "rb") as update:
        update_bytes = update.read()

        # Apply the dbx update.
        result = vm.APIRequest("/1.0/debug/secureboot/:update", method="POST", body=update_bytes, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        vm.WaitExpectedLog("incus-osd", "Appending certificate SHA256:[0-9A-F]{64} to EFI variable dbx", regex=True)

        time.sleep(5)
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "System is ready")

        # Expect to get back nine SecureBoot certificates from the API.
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        certs = result["metadata"]["state"]["secure_boot_certificates"]
        if len(certs) != 9:
            raise IncusOSException("expected nine SecureBoot certificates to be present")

def testUpdateMetadata(vm, os_version):
    """Test verification of update metadata consumed by the images provider."""

    # At this point we expect to have a fresh IncusOS running with Incus installed.
    # The update index.sjson has been properly validated using the Update intermediate CA.

    # Sign the update index.sjson using an incorrect intermediate CA and expect to get an openssl verification error.
    subprocess.run(["./incus-osd/image-publisher", "demote", "./local-image-server/", os_version, "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/update-E1.key", "SIG_CERTIFICATE": "./certs/update-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/support-E1.crt"}, capture_output=True, check=True)

    # Trigger an update check
    result = vm.APIRequest("/1.0/system/update/:check", method="POST")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    vm.WaitExpectedLog("incus-osd", "Failed to check for Secure Boot key updates err=unable to verify S/MIME message due to its use of a missing or unverifiable CA")

    # Clear out the journal logs
    vm.RunCommand("journalctl", "--rotate")
    vm.RunCommand("journalctl", "--vacuum-time=1ms")

    # Sign the update index.sjson with both an incorrect certificate and intermediate CA which will result
    # in a valid signature, but expect IncusOS to properly catch and return an error.
    subprocess.run(["./incus-osd/image-publisher", "promote", "./local-image-server/", os_version, "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/support-E1.key", "SIG_CERTIFICATE": "./certs/support-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/support-E1.crt"}, capture_output=True, check=True)

    # Trigger an update check
    result = vm.APIRequest("/1.0/system/update/:check", method="POST")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    vm.WaitExpectedLog("incus-osd", "Failed to check for Secure Boot key updates err=S/MIME message contained a valid signature, but was not signed by one of the following expected intermediate CAs: 'CN=TestOS - Update E1,O=TestOS'")

    # Clear out the journal logs
    vm.RunCommand("journalctl", "--rotate")
    vm.RunCommand("journalctl", "--vacuum-time=1ms")

    # Restore the update index.sjson to a correct state
    subprocess.run(["./incus-osd/image-publisher", "demote", "./local-image-server/", os_version, "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/update-E1.key", "SIG_CERTIFICATE": "./certs/update-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/update-E1.crt"}, capture_output=True, check=True)
    subprocess.run(["./incus-osd/image-publisher", "promote", "./local-image-server/", os_version, "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/update-E1.key", "SIG_CERTIFICATE": "./certs/update-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/update-E1.crt"}, capture_output=True, check=True)

def testHotfixScriptAPI(vm):
    """Test verification and running of a hotfix script via the debug API."""

    # Input that doesn't look like a S/MIME-signed message should be rejected.
    result = vm.APIRequest("/1.0/debug/:run-script", method="POST", body="#!/bin/sh\necho 'unsigned script'\n")
    if result["status_code"] == 200:
        raise IncusOSException("unexpected success, should have received an error")

    if result["error"] != "doesn't look like S/MIME-signed input\n\n":
        raise IncusOSException("got an unexpected error: " + result["error"])

    # Prepare a simple hotfix script and verify that it runs as expected.
    signed_script = subprocess.run(["openssl", "smime", "-sign", "-signer", "./certs/support-E1.crt", "-inkey", "./certs/support-E1.key", "-certfile", "./incus-osd/certs/files/support-E1.crt", "-text"], input="#!/bin/sh\necho 'Hello from hotfix script API'\n".encode("utf-8"), capture_output=True, check=True)

    result = vm.APIRequest("/1.0/debug/:run-script", method="POST", body=signed_script.stdout)
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    if result["metadata"] != "Hello from hotfix script API\n":
        raise IncusOSException("failed to run hotfix script, got: " + result["metadata"])

    # Prepare a simple hotfix script but sign it with an incorrect intermediate CA and expect to get an openssl verification error.
    signed_script = subprocess.run(["openssl", "smime", "-sign", "-signer", "./certs/support-E1.crt", "-inkey", "./certs/support-E1.key", "-certfile", "./incus-osd/certs/files/update-E1.crt", "-text"], input="#!/bin/sh\necho 'Hello from hotfix script API'\n".encode("utf-8"), capture_output=True, check=True)

    result = vm.APIRequest("/1.0/debug/:run-script", method="POST", body=signed_script.stdout)
    if result["status_code"] == 200:
        raise IncusOSException("unexpected success, should have received an error")

    if result["error"] != "unable to verify S/MIME message due to its use of a missing or unverifiable CA\n\n":
        raise IncusOSException("got an unexpected error: " + result["error"])

    # Prepare a simple hotfix script but sign it with both an incorrect certificate and intermediate CA which will result
    # in a valid signature, but expect IncusOS to properly catch and return an error.
    signed_script = subprocess.run(["openssl", "smime", "-sign", "-signer", "./certs/update-E1.crt", "-inkey", "./certs/update-E1.key", "-certfile", "./incus-osd/certs/files/update-E1.crt", "-text"], input="#!/bin/sh\necho 'Hello from hotfix script API'\n".encode("utf-8"), capture_output=True, check=True)

    result = vm.APIRequest("/1.0/debug/:run-script", method="POST", body=signed_script.stdout)
    if result["status_code"] == 200:
        raise IncusOSException("unexpected success, should have received an error")

    if result["error"] != "S/MIME message contained a valid signature, but was not signed by one of the following expected intermediate CAs: 'CN=TestOS - Support E1,O=TestOS'\n\n":
        raise IncusOSException("got an unexpected error: " + result["error"])

def testHotfixScriptRecovery(vm):
    """Test verification and running of a hotfix script from recovery media."""

    # Note that we only test a simple success case here, since running a hotfix script
    # via the debug API exercises the same code path for various errors.

    with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as recovery_img:
            # Prepare a simple hotfix script and verify that it runs as expected.
            signed_script = subprocess.run(["openssl", "smime", "-sign", "-out", os.path.join(tmp_dir, "hotfix.sh.sig"), "-signer", "./certs/support-E1.crt", "-inkey", "./certs/support-E1.key", "-certfile", "./incus-osd/certs/files/support-E1.crt", "-text"], input="#!/bin/sh\necho 'Hello from hotfix script media'\n".encode("utf-8"), capture_output=True, check=True)

            # Create a vfat partition labeled RESCUE_DATA and copy the hotfix script.
            util._create_user_media(recovery_img, tmp_dir, "img", 4*1024*1024*1024, "RESCUE_DATA")

            # Stop the VM and attach the recovery media.
            vm.StopVM()
            vm.AddDevice("recovery", "disk", "source="+recovery_img.name, "io.bus=usb")

            # Start the VM and wait for the recovery script to run.
            vm.StartVM()
            vm.WaitAgentRunning()
            vm.WaitExpectedLog("incus-osd", "Recovery partition detected")
            vm.WaitExpectedLog("incus-osd", "Hotfix script detected, verifying signature")
            vm.WaitExpectedLog("incus-osd", "Running hotfix script")
            vm.WaitExpectedLog("incus-osd", "Hotfix script completed output=Hello from hotfix script media")
            vm.WaitExpectedLog("incus-osd", "Recovery actions completed")
