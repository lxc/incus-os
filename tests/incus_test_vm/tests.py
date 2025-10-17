from inspect import getmembers, isfunction

from . import tests_incusos_api, tests_incusos_api_applications, tests_incusos_api_debug, tests_incusos_api_services, \
    tests_incusos_api_system, tests_incusos_api_system_logging, tests_incusos_api_system_network, \
    tests_incusos_api_system_provider, tests_incusos_api_system_resources, tests_incusos_api_system_security, \
    tests_incusos_api_system_storage, tests_incusos_api_system_update, tests_install_external_seed, tests_install_smoke, \
    tests_install_system_checks, tests_recovery, tests_secureboot_key_rotation, tests_seed_applications, tests_seed_install, \
    tests_upgrade

def GetTests(image_img, image_iso, old_image_img):
    ret = []
    """
    # Basic system pre-install checks
    for name, fn in getmembers(tests_install_system_checks, isfunction):
        ret.append([name + "/img", fn, image_img])

    # Baseline install smoke tests
    for name, fn in getmembers(tests_install_smoke, isfunction):
        ret.append([name + "/img", fn, image_img])
        ret.append([name + "/iso", fn, image_iso])

    # Basic application seed tests
    for name, fn in getmembers(tests_seed_applications, isfunction):
        ret.append([name + "/img", fn, image_img])

    # Basic install seed tests
    for name, fn in getmembers(tests_seed_install, isfunction):
        ret.append([name + "/img", fn, image_img])

    # External install seed tests
    for name, fn in getmembers(tests_install_external_seed, isfunction):
        ret.append([name + "/img", fn, image_img])

    # Basic system upgrade tests
    for name, fn in getmembers(tests_upgrade, isfunction):
        ret.append([name + "/img", fn, old_image_img])

    # IncusOS API tests
    for name, fn in getmembers(tests_incusos_api, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_applications, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_debug, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_services, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system, isfunction):
        ret.append([name + "/img", fn, image_img])
    """
    for name, fn in getmembers(tests_incusos_api_system_logging, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system_network, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system_provider, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system_resources, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system_security, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system_storage, isfunction):
        ret.append([name + "/img", fn, image_img])
    for name, fn in getmembers(tests_incusos_api_system_update, isfunction):
        ret.append([name + "/img", fn, image_img])
    """
    # SecureBoot key rotation tests
    for name, fn in getmembers(tests_secureboot_key_rotation, isfunction):
        ret.append([name + "/img", fn, image_img])

    # Recovery mode tests
    for name, fn in getmembers(tests_recovery, isfunction):
        ret.append([name + "/img", fn, image_img])
    """
    return ret
