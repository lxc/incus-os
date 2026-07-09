import os
import subprocess
import tempfile

from .incus_test_vm import IncusOSException, util

def TestUpdateMetadata(vm):
    """Test verification of update metadata consumed by the images provider."""

    print("  Running provider update metadata tests", flush=True)

    # At this point we expect to have a fresh IncusOS running with Incus installed.
    # The update index.sjson has been properly validated using the Update intermediate CA.

    # Sign the update index.sjson using an incorrect intermediate CA and expect to get an openssl verification error.
    subprocess.run(["./incus-osd/image-publisher", "demote", "./local-image-server/", os.environ["VERSION"], "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/update-E1.key", "SIG_CERTIFICATE": "./certs/update-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/support-E1.crt"}, capture_output=True, check=True)

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
    subprocess.run(["./incus-osd/image-publisher", "promote", "./local-image-server/", os.environ["VERSION"], "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/support-E1.key", "SIG_CERTIFICATE": "./certs/support-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/support-E1.crt"}, capture_output=True, check=True)

    # Trigger an update check
    result = vm.APIRequest("/1.0/system/update/:check", method="POST")
    if result["status_code"] != 200:
        raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

    vm.WaitExpectedLog("incus-osd", "Failed to check for Secure Boot key updates err=S/MIME message contained a valid signature, but was not signed by one of the following expected intermediate CAs: 'CN=TestOS - Update E1,O=TestOS'")

    # Clear out the journal logs
    vm.RunCommand("journalctl", "--rotate")
    vm.RunCommand("journalctl", "--vacuum-time=1ms")

    # Restore the update index.sjson to a correct state
    subprocess.run(["./incus-osd/image-publisher", "demote", "./local-image-server/", os.environ["VERSION"], "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/update-E1.key", "SIG_CERTIFICATE": "./certs/update-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/update-E1.crt"}, capture_output=True, check=True)
    subprocess.run(["./incus-osd/image-publisher", "promote", "./local-image-server/", os.environ["VERSION"], "stable"], env={"PATH": "/usr/bin", "SIG_KEY": "./certs/update-E1.key", "SIG_CERTIFICATE": "./certs/update-E1.crt", "SIG_CHAIN": "./incus-osd/certs/files/update-E1.crt"}, capture_output=True, check=True)

def TestScriptAPI(vm):
    """Test verification and running of a hotfix script via the debug API."""

    print("  Running hotfix script via debug API tests", flush=True)

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

def TestScriptRecovery(vm):
    """Test verification and running of a hotfix script from recovery media."""

    print("  Running hotfix script via recovery media tests", flush=True)

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
