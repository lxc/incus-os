import json

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestInstallUseSWTPM(install_image):
    test_name = "use-swtpm"
    test_seed = {
        "install.json": """{"security":{"missing_tpm":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.RemoveDevice("vtpm")

        vm.WaitSystemReady(incusos_version)

        # Should see a log message about swtpm
        vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")

        # Check some PCR values: expect PCR0 to be empty with swtpm, while PCR7, PCR11, and PCR15 should have non-zero values
        result = vm.RunCommand("tpm2_pcrread", "sha256:0")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" not in str(result.stdout):
            raise IncusOSException("PCR0 has a non-zero value")

        result = vm.RunCommand("tpm2_pcrread", "sha256:7")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" in str(result.stdout):
            raise IncusOSException("PCR7 isn't initialized")

        result = vm.RunCommand("tpm2_pcrread", "sha256:11")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" in str(result.stdout):
            raise IncusOSException("PCR11 isn't initialized")

        result = vm.RunCommand("tpm2_pcrread", "sha256:15")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" in str(result.stdout):
            raise IncusOSException("PCR15 isn't initialized")

        # Verify that LUKS encryption is bound to PCRs 7+11+15
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part9")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 7+11+15")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part10")
        if "tpm2-hash-pcrs:   7+15" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 7+11+15")

        # Verify the security endpoint reflects swtpm is in use
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["state"]["tpm_status"] != "swtpm":
            raise IncusOSException("tpm_status != swtpm, got " + result["metadata"]["state"]["tpm_status"])

        if result["metadata"]["state"]["system_state_is_trusted"]:
            raise IncusOSException("expected to see system state is untrusted")

        # Set a different encryption recovery key.
        result["metadata"]["config"]["encryption_recovery_keys"][0] = "foo-bar-biz-1234"
        result = vm.APIRequest("/1.0/system/security", method="PUT", body=json.dumps(result["metadata"]))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))
