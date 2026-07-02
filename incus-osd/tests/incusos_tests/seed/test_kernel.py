from ..incus_test_vm import IncusTestVM, IncusOSException, util

def TestSeedKernel(install_image):
    test_name = "seed-kernel"
    test_seed = {
        "install.json": "{}",
        "kernel.json": """{"console":[{"device":"/dev/ttyS0","baud_rate":115200}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Verify the baud rate has been updated from the default of 9600
        output = vm.RunCommand("/usr/bin/stty", "-F", "/dev/ttyS0", "speed")

        if output.stdout.decode("utf-8").strip() != "115200":
            raise IncusOSException("baud rate for /dev/ttyS0 != 115200")
