from ..incus_test_vm import IncusTestVM, IncusOSException, util

def TestSeedUpdate(install_image):
    test_name = "seed-update"
    test_seed = {
        "install.json": "{}",
        "update.json": """{"auto_reboot":true,"channel":"testing","check_frequency":"1h","maintenance_windows":[{"start_hour":6,"start_minute":15,"end_hour":16,"end_minute":30}]}"""
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version, channel="testing")

        # Check that we get back the expected update configuration.
        result = vm.APIRequest("/1.0/system/update")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if not result["metadata"]["config"]["auto_reboot"]:
            raise IncusOSException("expected auto_reboot to be true")

        if result["metadata"]["config"]["channel"] != "testing":
            raise IncusOSException("expected channel to be 'testing'")

        if result["metadata"]["config"]["check_frequency"] != "1h":
            raise IncusOSException("expected check_frequency to be '1h'")

        if len(result["metadata"]["config"]["maintenance_windows"]) != 1:
            raise IncusOSException("expected a single maintenance window to be defined")

        if result["metadata"]["config"]["maintenance_windows"][0]["start_hour"] != 6:
            raise IncusOSException("expected start_hour to be 6")

        if result["metadata"]["config"]["maintenance_windows"][0]["start_minute"] != 15:
            raise IncusOSException("expected start_minute to be 15")

        if result["metadata"]["config"]["maintenance_windows"][0]["end_hour"] != 16:
            raise IncusOSException("expected end_hour to be 16")

        if result["metadata"]["config"]["maintenance_windows"][0]["end_minute"] != 30:
            raise IncusOSException("expected end_minute to be 30")
