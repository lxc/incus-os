#!/usr/bin/python3

import os

from common import _check_deps
from incusos_tests.incus_test_vm import IncusTestVM, util
from incusos_tests.tests_certificate_e2e import TestUpdateMetadata, TestScriptAPI, TestScriptRecovery

_check_deps()

# Symlink the raw IncusOS image so it has the expected name
install_image = os.path.join(os.getcwd(), os.environ["OSNAME"] + "_" + os.environ["VERSION"] + ".img")
os.symlink(os.path.join(os.getcwd(), "mkosi.output", os.environ["OSNAME"] + "_" + os.environ["VERSION"] + ".raw"), install_image)

# Create the test VM
test_name = "certificate-e2e-tests"
test_seed = {
    "install.json": "{}",
}

print("Beginning end-to-end certificate tests", flush=True)

test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
    vm.WaitSystemReady(os_version)

    TestUpdateMetadata(vm)

    TestScriptAPI(vm)

    TestScriptRecovery(vm)

print("End-to-end certificate tests completed successfully", flush=True)
