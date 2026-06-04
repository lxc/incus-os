import io
import os
import re
import subprocess
import tarfile
import tempfile

from .incus_test_vm import IncusTestVM, IncusOSException

def TestFlasherToolStableIMG(_):
    test_name = "flasher-tool-stable-img"

    os_name, test_image = _flasher_download_image("stable", "img")

    with IncusTestVM(os_name, test_name, test_image, "") as vm:
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_1-0000:00:01.0:00.6-4-0:0 target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

def TestFlasherToolTestingISO(_):
    test_name = "flasher-tool-testing-iso"

    os_name, test_image = _flasher_download_image("testing", "iso")

    with IncusTestVM(os_name, test_name, test_image, "") as vm:
        vm.StartVM()
        vm.WaitAgentRunning()
        vm.WaitExpectedLog("incus-osd", "Installing " + os_name + " source=/dev/disk/by-id/scsi-0QEMU_QEMU_CD-ROM_incus_boot--media target=/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root")
        vm.WaitExpectedLog("incus-osd", os_name + " was successfully installed")

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

        match = re.search("Downloading and decompressing (.+) image \\(" + image_format + "\\) version (\\d+) from Linux Containers CDN", str(result.stderr))
        if not match:
            raise IncusOSException("Failed to download image")

        os.rename(os.path.join(tmp_dir, match.group(1) + "_" + match.group(2) + "." + image_format), "flasher-install-image." + image_format)

        return match.group(1), os.path.join(os.getcwd(), "flasher-install-image." + image_format)
