from .incus_test_vm import IncusTestVM, util

def TestInstallNoTPMNoSWTPM(install_image):
    test_name = "no-tpm-no-swtpm"
    test_seed = {
        "install.json": "{}"
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.RemoveDevice("vtpm")

        # Verify we get expected error
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "no physical TPM found, and install seed doesn't allow for use of swtpm")

def TestInstallUseSWTPM(install_image):
    test_name = "use-swtpm"
    test_seed = {
        "install.json": """{"use_swtpm":true}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.RemoveDevice("vtpm")

        vm.WaitSystemReady(incusos_version)

        # Should see a log message about swtpm
        vm.WaitExpectedLog("incus-osd", "No physical TPM found, using swtpm")

        # Verify the security endpoint reflects swtpm is in use
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if result["metadata"]["state"]["tpm_status"] != "swtpm":
            raise IncusOSException("tpm_status != swtpm, got " + result["metadata"]["state"]["tpm_status"])

        # Check some PCR values: expect PCR0 to be empty with swtpm, while PCR7 and PCR11 should have non-zero values
        result = vm.RunCommand("tpm2_pcrread", "sha256:0")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" not in str(result.stdout):
            raise IncusOSException("PCR0 has a non-zero value")

        result = vm.RunCommand("tpm2_pcrread", "sha256:7")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" in str(result.stdout):
            raise IncusOSException("PCR7 isn't initialized")

        result = vm.RunCommand("tpm2_pcrread", "sha256:11")
        if "0x0000000000000000000000000000000000000000000000000000000000000000" in str(result.stdout):
            raise IncusOSException("PCR11 isn't initialized")

        # Set a different encryption recovery key.
        result["metadata"]["config"]["encryption_recovery_keys"][0] = "foo-bar-biz-1234"
        result = vm.APIRequest("/1.0/system/security", method="PUT", body=json.dumps(result["metadata"]))
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))
