from inspect import getmembers, isfunction

from . import tests_flasher_tool, tests_incusos_api, tests_incusos_api_applications, tests_incusos_api_debug, \
    tests_incusos_api_services, tests_incusos_api_system, tests_incusos_api_system_logging, tests_incusos_api_system_provider, \
    tests_incusos_api_system_resources, tests_incusos_api_system_security, tests_incusos_api_system_storage_import_pool, \
    tests_incusos_api_system_storage_local_pool, tests_incusos_live, tests_install_smoke, tests_install_system_checks, \
    tests_seed_applications, tests_seed_install, tests_upgrade

class IncusOSTests:
    def __init__(self, prior_image_img, current_image_img, current_image_iso):
        self.prior_image_img = prior_image_img
        self.current_image_img = current_image_img
        self.current_image_iso = current_image_iso

    def GetTests(self):
        # Basic system upgrade tests that depend on starting with a release prior to the current one
        upgrade_tests = [tests_upgrade]

        # Tests that rely on the ISO install image
        iso_install_tests = [tests_install_smoke]

        # The bulk of the tests for IncusOS
        core_tests = [
            # Basic system pre-install checks
            tests_install_system_checks,

            # Baseline install smoke tests
            tests_install_smoke,

            # Basic application seed tests
            tests_seed_applications,

            # Basic install seed tests
            tests_seed_install,

            # Test running IncusOS live from a large enough drive
            tests_incusos_live,

            # IncusOS API tests
            tests_incusos_api,
            tests_incusos_api_applications,
            tests_incusos_api_debug,
            tests_incusos_api_services,
            tests_incusos_api_system,
            tests_incusos_api_system_logging,
            tests_incusos_api_system_provider,
            tests_incusos_api_system_resources,
            tests_incusos_api_system_security,
            tests_incusos_api_system_storage_import_pool,
            tests_incusos_api_system_storage_local_pool,
        ]

        # Test the flasher-tool utility
        flasher_tool_tests = [tests_flasher_tool]

        ret = []

        ret.extend(self._get_tests(core_tests, self.current_image_img))
        ret.extend(self._get_tests(upgrade_tests, self.prior_image_img))
        ret.extend(self._get_tests(iso_install_tests, self.current_image_iso))
        ret.extend(self._get_tests(flasher_tool_tests, ""))

        return ret

    def _get_tests(self, modules, image):
        ext = "img" if image.endswith(".img") else "iso"

        ret = []

        for mod in modules:
            for name, fn in getmembers(mod, isfunction):
                if not name.startswith("Test"):
                    continue

                ret.append([name + "/" + ext, fn, image])

        return ret
