from . import IncusTestVM, util

def TestIncusOSAPIApplications(install_image):
    test_name = "incusos-api-applications"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Test top-level /1.0/applications endpoint.
        result = vm.APIRequest("/1.0/applications")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]) != 1:
            raise Exception("expected exactly one application")

        if result["metadata"][0] != "/1.0/applications/incus":
            raise Exception("expected the incus application to be installed")

        # Get current application state
        result = vm.APIRequest("/1.0/applications/incus")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        state = result["metadata"]["state"]
        if not state["initialized"]:
            raise Exception("incus application isn't initialized")

        if state["version"] != incusos_version:
            raise Exception("incus application version mismatch (%s vs %s)" % (state["version"], incusos_version))

        # For testing, create an empty test file that should be included in the backup/restore steps
        result = vm.RunCommand("touch", "/var/lib/incus/test-file")

        # Generate a backup for the incus application
        backup_archive = vm.APIRequest("/1.0/applications/incus/:backup", method="POST", return_raw_content=True)

        # Trigger a factory-reset
        result = vm.APIRequest("/1.0/applications/incus/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify our test file is no longer present
        result = vm.RunCommand("stat", "/var/lib/incus/test-file", check=False)
        if result.returncode == 0:
            raise Exception("file /var/lib/incus/test-file shouldn't exist")

        # Restore the application backup
        result = vm.APIRequest("/1.0/applications/incus/:restore", method="POST", body=backup_archive, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify our test file is restored
        result = vm.RunCommand("stat", "/var/lib/incus/test-file")
