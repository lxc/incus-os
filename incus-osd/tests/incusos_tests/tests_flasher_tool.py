import io
import os
import re
import subprocess
import tarfile
import tempfile

from .incus_test_vm import IncusTestVM

def TestFlasherToolStableIMG(_):
    test_name = "flasher-tool-stable-img"

    test_image = _flasher_download_image("stable", "img")

    with IncusTestVM(test_name, test_image) as vm:
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/sdb target=/dev/sda")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

def TestFlasherToolTestingISO(_):
    test_name = "flasher-tool-testing-iso"

    test_image = _flasher_download_image("testing", "iso")

    with IncusTestVM(test_name, test_image) as vm:
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing IncusOS source=/dev/mapper/sr0 target=/dev/sda")
        vm.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

def _flasher_download_image(channel, image_format):
    if not os.path.exists("./incus-osd/flasher-tool"):
        subprocess.run(["go", "build", "./cmd/flasher-tool/"], cwd=os.path.join(os.getcwd(), "incus-osd"), capture_output=True, check=True)

    with tempfile.TemporaryDirectory(dir=os.getcwd()) as tmp_dir:
        with tarfile.open(os.path.join(tmp_dir, "seed.tar"), mode="w") as tar:
            raw = "force_reboot: true".encode("utf-8")
            buf = io.BytesIO(raw)
            ti = tarfile.TarInfo(name="install.yaml")
            ti.size = len(raw)
            tar.addfile(ti, buf)

        result = subprocess.run(["../incus-osd/flasher-tool", "--channel", channel, "--format", image_format, "--seed", "seed.tar"], cwd=tmp_dir, capture_output=True, check=True)

        match = re.search("Downloading and decompressing IncusOS image \\(" + image_format + "\\) version (\\d+) from Linux Containers CDN", str(result.stderr))
        if not match:
            raise Exception("Failed to download image")

        os.rename(os.path.join(tmp_dir, "IncusOS_" + match.group(1) + "." + image_format), "flasher-install-image." + image_format)

        return os.path.join(os.getcwd(), "flasher-install-image." + image_format)
