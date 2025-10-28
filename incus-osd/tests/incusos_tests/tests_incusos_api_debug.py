from .incus_test_vm import IncusTestVM, util

def TestIncusOSAPIDebug(install_image):
    test_name = "incusos-api-debug"
    test_seed = {
        "install.json": "{}",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Test top-level /1.0/debug endpoint.
        result = vm.APIRequest("/1.0/debug")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]) != 2:
            raise Exception("expected two debug endpoints")

        for endpoint in ["/1.0/debug/log", "/1.0/debug/tui"]:
            if endpoint not in result["metadata"]:
                raise Exception(f"missing expected endpoint {endpoint}")

        # Test journal retrieval.
        result = vm.APIRequest("/1.0/debug/log?unit=incus-osd")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        lines = [l["MESSAGE"] for l in result["metadata"]]
        if "INFO System is ready" not in lines[-1]:
            raise Exception("got unexpected final log line: '" + lines[-1] + "'")

        # Test sending log messages via API.
        result = vm.APIRequest("/1.0/debug/tui/:write-message", method="POST", body='{"level":"INFO","message":"Test message one"}')
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))
        result = vm.APIRequest("/1.0/debug/tui/:write-message", method="POST", body='{"level":"WARN","message":"Test message two"}')
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))
        result = vm.APIRequest("/1.0/debug/tui/:write-message", method="POST", body='{"level":"ERROR","message":"Test message three"}')
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        result = vm.APIRequest("/1.0/debug/log?unit=incus-osd")
        if result["status_code"] != 200:
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Get the log messages back.
        lines = [l["MESSAGE"] for l in result["metadata"]]
        if "INFO Test message one" not in lines[-3]:
            raise Exception("failed to get INFO log message back")
        if "WARN Test message two" not in lines[-2]:
            raise Exception("failed to get WARN log message back")
        if "ERROR Test message three" not in lines[-1]:
            raise Exception("failed to get ERROR log message back")
