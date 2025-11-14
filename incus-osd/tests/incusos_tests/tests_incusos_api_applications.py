from .incus_test_vm import IncusTestVM, util

def TestIncusOSAPIApplicationsIncus(install_image):
    test_name = "incusos-api-applications-incus"
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
        vm.RunCommand("touch", "/var/lib/incus/test-file")

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
        vm.RunCommand("stat", "/var/lib/incus/test-file")

def TestIncusOSAPIApplicationsMigrationManager(install_image):
    test_name = "incusos-api-applications-migration-manager"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"migration-manager"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="migration-manager")

        # Test top-level /1.0/applications endpoint.
        result = vm.APIRequest("/1.0/applications")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]) != 1:
            raise Exception("expected exactly one application")

        if result["metadata"][0] != "/1.0/applications/migration-manager":
            raise Exception("expected the migration manager application to be installed")

        # Get current application state
        result = vm.APIRequest("/1.0/applications/migration-manager")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        state = result["metadata"]["state"]
        if not state["initialized"]:
            raise Exception("migration manager application isn't initialized")

        if state["version"] != incusos_version:
            raise Exception("migration manager application version mismatch (%s vs %s)" % (state["version"], incusos_version))

        # For testing, create an empty test file that should be included in the backup/restore steps
        vm.RunCommand("touch", "/var/lib/migration-manager/test-file")

        # Generate a backup for the migration manager application
        backup_archive = vm.APIRequest("/1.0/applications/migration-manager/:backup", method="POST", return_raw_content=True)

        # Trigger a factory-reset
        result = vm.APIRequest("/1.0/applications/migration-manager/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify our test file is no longer present
        result = vm.RunCommand("stat", "/var/lib/migration-manager/test-file", check=False)
        if result.returncode == 0:
            raise Exception("file /var/lib/migration-manager/test-file shouldn't exist")

        # Restore the application backup
        result = vm.APIRequest("/1.0/applications/migration-manager/:restore", method="POST", body=backup_archive, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify our test file is restored
        vm.RunCommand("stat", "/var/lib/migration-manager/test-file")

def TestIncusOSAPIApplicationsOperationsCenter(install_image):
    test_name = "incusos-api-applications-operations-center"
    test_seed = {
        "install.json": "{}",
        "applications.json": """{"applications":[{"name":"operations-center"}]}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version, application="operations-center")

        # Test top-level /1.0/applications endpoint.
        result = vm.APIRequest("/1.0/applications")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]) != 1:
            raise Exception("expected exactly one application")

        if result["metadata"][0] != "/1.0/applications/operations-center":
            raise Exception("expected the operations center application to be installed")

        # Get current application state
        result = vm.APIRequest("/1.0/applications/operations-center")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        state = result["metadata"]["state"]
        if not state["initialized"]:
            raise Exception("operations center application isn't initialized")

        if state["version"] != incusos_version:
            raise Exception("operations center application version mismatch (%s vs %s)" % (state["version"], incusos_version))

        # For testing, create an empty test file that should be included in the backup/restore steps
        vm.RunCommand("touch", "/var/lib/operations-center/test-file")

        # Generate a backup for the operations center application
        backup_archive = vm.APIRequest("/1.0/applications/operations-center/:backup", method="POST", return_raw_content=True)

        # Trigger a factory-reset
        result = vm.APIRequest("/1.0/applications/operations-center/:factory-reset", method="POST")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify our test file is no longer present
        result = vm.RunCommand("stat", "/var/lib/operations-center/test-file", check=False)
        if result.returncode == 0:
            raise Exception("file /var/lib/operations-center/test-file shouldn't exist")

        # Restore the application backup
        result = vm.APIRequest("/1.0/applications/operations-center/:restore", method="POST", body=backup_archive, content_type="application/x-tar")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Verify our test file is restored
        vm.RunCommand("stat", "/var/lib/operations-center/test-file")
