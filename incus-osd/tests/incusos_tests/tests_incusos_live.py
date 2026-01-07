import subprocess
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSLive(install_image):
    test_name = "incusos-live"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, root_size="1MiB") as vm:
        # Remove the install image, enlarge its size and re-attach it
        vm.RemoveDevice("boot-media")

        subprocess.run(["/usr/bin/truncate", "--size", "50GiB", test_image], capture_output=True, check=True)

        vm.AddDevice("live-image", "disk", "source="+test_image, "boot.priority=10")

        # Start the VM and expect IncusOS to start running immediately
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Shouldn't see any mention of a degraded security state
        vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestIncusOSLiveSWTPM(install_image):
    test_name = "incusos-live-swtpm"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image, root_size="1MiB") as vm:
        # Remove the install image, enlarge its size and re-attach it
        vm.RemoveDevice("boot-media")

        subprocess.run(["/usr/bin/truncate", "--size", "50GiB", test_image], capture_output=True, check=True)

        vm.AddDevice("live-image", "disk", "source="+test_image, "boot.priority=10")

        # Remove the tpm
        vm.RemoveDevice("vtpm")

        # Start the VM and expect swtpm configuration to happen
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Configuring swtpm-backed TPM on first boot, restarting in five seconds")
        vm.LogDoesntContain("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")

        # After auto-reboot, expect IncusOS to start running immediately
        time.sleep(10)
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

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

def TestIncusOSLiveNoSecureBoot(install_image):
    test_name = "incusos-live-no-secure-boot"
    test_seed = None

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)
    util._remove_secureboot_keys(test_image)

    with IncusTestVM(test_name, test_image, root_size="1MiB") as vm:
        # Remove the install image, enlarge its size and re-attach it
        vm.RemoveDevice("boot-media")

        subprocess.run(["/usr/bin/truncate", "--size", "50GiB", test_image], capture_output=True, check=True)

        vm.AddDevice("live-image", "disk", "source="+test_image, "boot.priority=10")

        # Start the VM and expect IncusOS to start running immediately
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        vm.WaitExpectedLog("incus-osd", "Degraded security state: Secure Boot is disabled")
        vm.WaitExpectedLog("incus-osd", "Downloading application update application=incus version="+incusos_version)
        vm.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

        # Verify that LUKS encryption is bound to PCRs 4+7+11
        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/sdb9")
        if "tpm2-hash-pcrs:   4+7" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS swap partition not properly bound to PCRs 4+7+11")

        result = vm.RunCommand("cryptsetup", "luksDump", "/dev/sdb10")
        if "tpm2-hash-pcrs:   4+7" not in str(result.stdout) or "tpm2-pubkey-pcrs: 11" not in str(result.stdout):
            raise IncusOSException("LUKS root partition not properly bound to PCRs 4+7+11")
