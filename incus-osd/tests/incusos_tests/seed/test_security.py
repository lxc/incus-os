from ..incus_test_vm import IncusTestVM, IncusOSException, util

def TestSeedSecurity(install_image):
    test_name = "seed-security"
    test_seed = {
        "install.json": "{}",
        "security.json": """{"custom_ca_certs":["-----BEGIN CERTIFICATE-----\\nMIIBrTCCAVOgAwIBAgIUSzoVuJh5V+uJMeJxYh6K+YoCBXMwCgYIKoZIzj0EAwMw\\nLDEZMBcGA1UEAwwQVGVzdE9TIC0gUm9vdCBFMTEPMA0GA1UECgwGVGVzdE9TMB4X\\nDTI2MDYyMzE0MjkzN1oXDTM2MDYyMDE0MjkzN1owLDEZMBcGA1UEAwwQVGVzdE9T\\nIC0gUm9vdCBFMTEPMA0GA1UECgwGVGVzdE9TMFkwEwYHKoZIzj0CAQYIKoZIzj0D\\nAQcDQgAEnCNObq07+z6WBwEIfMPKGrNHD0GTeDiSm371JfwQUz2P2YmZrDXBIgkg\\nAsFcF2fMNie5x+CVtpOR6ybkrxCvq6NTMFEwHQYDVR0OBBYEFDXTcCO5mk85DELG\\nx0iAmpivf2ErMB8GA1UdIwQYMBaAFDXTcCO5mk85DELGx0iAmpivf2ErMA8GA1Ud\\nEwEB/wQFMAMBAf8wCgYIKoZIzj0EAwMDSAAwRQIgT+n1+wuRCfFb8ZSJDTwGPYZA\\ng87AdIeLEcQ3x/uZpgoCIQD2Wjk/L4HdWJn0OlipAsLww6mTWq4EndK/2NQQRlam\\nCw==\\n-----END CERTIFICATE-----\\n"]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Verify IncusOS found and applied the custom CA certificate.
        vm.WaitExpectedLog("incus-osd", "Custom CAs configured from security seed, restarting incus-osd daemon")

        # Check that we get back the expected security configuration.
        result = vm.APIRequest("/1.0/system/security")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if len(result["metadata"]["config"]["custom_ca_certs"]) != 1:
            raise IncusOSException("expected exactly one custom CA certificate")

        # Verify the custom CA certificate is present in the system store.
        vm.RunCommand("grep", "AQcDQgAEnCNObq07+z6WBwEIfMPKGrNHD0GTeDiSm371JfwQUz2P2YmZrDXBIgkg", "/etc/ssl/certs/ca-certificates.crt")

def TestSeedSecurityInvalid(install_image):
    test_name = "seed-security-invalid"
    test_seed = {
        "install.json": "{}",
        "security.json": """{"encryption_recovery_keys":["my-password"]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        # Perform IncusOS install.
        vm.StartVM()
        vm.WaitAgentRunning()

        # Verify attempting to set encryption recovery keys via the seed is rejected.
        vm.WaitExpectedLog("incus-osd", "it is not possible to set encryption recovery key(s) via the security seed")
