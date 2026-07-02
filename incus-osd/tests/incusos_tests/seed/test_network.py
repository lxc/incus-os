from ..incus_test_vm import IncusTestVM, IncusOSException, util

def TestSeedNetwork(install_image):
    test_name = "seed-network"
    test_seed = {
        "install.json": "{}",
        "network.json": """{"dns":{"domain":"example.org","hostname":"myserver"},"time":{"timezone":"America/Denver"},"interfaces":[{"addresses":["dhcp4","slaac"],"name":"enp5s0","hwaddr":"enp5s0"}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Check that we get back the expected network configuration.
        result = vm.APIRequest("/1.0/system/network")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if len(result["metadata"]["state"]["interfaces"]) != 1:
            raise IncusOSException("expected a single network interface to exist")

        output = vm.RunCommand("timedatectl", "show")

        if "Timezone=America/Denver" not in output.stdout.decode("utf-8"):
            raise IncusOSException("system timezone doesn't match seed configuration")

        output = vm.RunCommand("hostnamectl", "hostname")

        if output.stdout.decode("utf-8").strip() != "myserver.example.org":
            raise IncusOSException("system hostname doesn't match seed configuration")
