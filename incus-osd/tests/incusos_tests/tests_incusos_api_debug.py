from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPIDebug(install_image):
    test_name = "incusos-api-debug"
    test_seed = {
        "install.json": "{}",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
        vm.WaitSystemReady(os_version)

        # Test top-level /1.0/debug endpoint.
        result = vm.APIRequest("/1.0/debug")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if len(result["metadata"]) != 6:
            raise IncusOSException("expected six debug endpoints")

        for endpoint in ["/1.0/debug/kernel", "/1.0/debug/log", "/1.0/debug/processes", "/1.0/debug/report", "/1.0/debug/:run-script", "/1.0/debug/secureboot"]:
            if endpoint not in result["metadata"]:
                raise IncusOSException(f"missing expected endpoint {endpoint}")

        # Test system metrics report.
        result = vm.APIRequest("/1.0/debug/report?match=io.systemd.Basic")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        if result["metadata"]["media_type"] != "application/vnd.io.systemd.report":
            raise IncusOSException("unexpected report media type: " + result["metadata"]["media_type"])

        metrics = {m["name"]: m["value"] for m in result["metadata"]["metrics"]}
        if not metrics.get("io.systemd.Basic.OSRelease.NAME").startswith(os_name):
            raise IncusOSException("missing or unexpected OS name metric in report")

        if any(not m["name"].startswith("io.systemd.Basic") for m in result["metadata"]["metrics"]):
            raise IncusOSException("report contains metrics outside of the requested family")

        # Test journal retrieval.
        result = vm.APIRequest("/1.0/debug/log?unit=incus-osd")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        lines = [l["MESSAGE"] for l in result["metadata"]]
        if "INFO System is ready" not in lines[-1]:
            raise IncusOSException("got unexpected final log line: '" + lines[-1] + "'")

        # Test sending log messages via API.
        result = vm.APIRequest("/internal/tui/:write-message", method="POST", body='{"level":"INFO","message":"Test message one"}', use_unix_socket=True)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))
        result = vm.APIRequest("/internal/tui/:write-message", method="POST", body='{"level":"WARN","message":"Test message two"}', use_unix_socket=True)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))
        result = vm.APIRequest("/internal/tui/:write-message", method="POST", body='{"level":"ERROR","message":"Test message three"}', use_unix_socket=True)
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        result = vm.APIRequest("/1.0/debug/log?unit=incus-osd")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

        # Get the log messages back.
        lines = [l["MESSAGE"] for l in result["metadata"]]
        if "INFO Test message one" not in lines[-3]:
            raise IncusOSException("failed to get INFO log message back")
        if "WARN Test message two" not in lines[-2]:
            raise IncusOSException("failed to get WARN log message back")
        if "ERROR Test message three" not in lines[-1]:
            raise IncusOSException("failed to get ERROR log message back")
