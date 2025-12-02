import json
import os
import re
import subprocess
import time

from . import util

class IncusOSException(Exception):
    """A custom exception raised by test code."""

    pass

class IncusTestVM:
    def __init__(self, vm_name_base, install_image, root_size="50GiB"):
        self.vm_name = vm_name_base + "-" + util._get_random_string()
        self.root_size = root_size
        self.install_image = install_image
        self.is_raw_image = self.install_image.endswith(".img")
        self.isos = []

    def __enter__(self):
        """Create an empty test VM with a basic configuration for running IncusOS."""

        if not self.is_raw_image:
            subprocess.run(["incus", "storage", "volume", "import", "default", self.install_image, self.vm_name+".iso", "--type=iso"], capture_output=True, check=True)

        subprocess.run(["incus", "init", "--empty", "--vm", self.vm_name, "-c", "security.secureboot=false", "-c", "limits.cpu=2", "-c", "limits.memory=4GiB", "-d", "root,size="+self.root_size], capture_output=True, check=True)
        self.AddDevice("vtpm", "tpm")

        if self.is_raw_image:
            self.AddDevice("boot-media", "disk", "source="+self.install_image, "io.bus=usb", "boot.priority=10", "readonly=false")
        else:
            self.AddDevice("boot-media", "disk", "pool=default", "source="+self.vm_name+".iso", "boot.priority=10")

        return self

    def __exit__(self, exc_type, exc_value, traceback):
        """Forcefully delete the VM and the install image, ignoring any errors."""

        subprocess.run(["incus", "delete", "-f", self.vm_name], capture_output=True)

        os.remove(self.install_image)
        if not self.is_raw_image:
            subprocess.run(["incus", "storage", "volume", "delete", "default", self.vm_name+".iso"], capture_output=True)

        for iso in self.isos:
            subprocess.run(["incus", "storage", "volume", "delete", "default", iso], capture_output=True)

    def AddDevice(self, device, *args):
        """Add a device to the VM."""

        subprocess.run(["incus", "config", "device", "add", self.vm_name, device, *args], capture_output=True, check=True)

    def AttachISO(self, iso_image, device_name):
        """Attach an ISO image to the VM."""

        source_name = self.vm_name+"-"+os.path.basename(iso_image)

        subprocess.run(["incus", "storage", "volume", "import", "default", iso_image, source_name, "--type=iso"], capture_output=True, check=True)
        self.AddDevice(device_name, "disk", "pool=default", "source="+source_name)

        self.isos.append(source_name)

    def RemoveDevice(self, device):
        """Remove a device from the VM."""

        subprocess.run(["incus", "config", "device", "remove", self.vm_name, device], capture_output=True, check=True)

    def SetDeviceProperty(self, device, prop):
        """Set/change a property for the VM's device."""

        subprocess.run(["incus", "config", "device", "set", self.vm_name, device, prop], capture_output=True, check=True)

    def StartVM(self, timeout=60):
        """Start the VM and wait up to 15 seconds by default for the command to return."""

        subprocess.run(["incus", "start", self.vm_name], capture_output=True, check=True, timeout=timeout)

    def StopVM(self, timeout=120):
        """Stop the VM and wait up to 60 seconds by default for the command to return."""

        subprocess.run(["incus", "stop", self.vm_name], capture_output=True, check=True, timeout=timeout)

    def WaitSystemReady(self, incusos_version, source="/dev/sdb", target="/dev/sda", application="incus", remove_devices=[]):
        """Wait for the system install to complete, the given application to be configured and the system become ready for use."""

        # Perform IncusOS install.
        self.StartVM()
        self.WaitAgentRunning()
        self.WaitExpectedLog("incus-osd", "Installing IncusOS source=" + source + " target=" + target, regex=True)
        self.WaitExpectedLog("incus-osd", "IncusOS was successfully installed")

        # Stop the VM post-install and remove install media.
        self.StopVM()
        self.RemoveDevice("boot-media")

        for device in remove_devices:
            self.RemoveDevice(device)

        # Start freshly installed IncusOS and verify successful boot.
        self.StartVM()
        self.WaitAgentRunning()
        self.WaitExpectedLog("incus-osd", "Auto-generating encryption recovery key, this may take a few seconds")
        self.WaitExpectedLog("incus-osd", "Downloading application application="+application+" version="+incusos_version)
        self.WaitExpectedLog("incus-osd", "System is ready version="+incusos_version)

    def WaitAgentRunning(self, timeout=300):
        """Wait for the Incus agent to start in the VM."""

        start = time.time()

        while True:
            try:
                subprocess.run(["incus", "exec", self.vm_name, "true"], capture_output=True, check=True)

                return
            except:
                pass

            if time.time() - start > timeout:
                raise IncusOSException("timed out waiting for agent to start")

            time.sleep(1)

    def WaitExpectedLog(self, unit, log, timeout=480, regex=False):
        """Wait for an expected log entry to appear in the VM."""

        start = time.time()

        while True:
            # Occasionally the querying of the journal exits with status 255. Don't raise an exception in this
            # case and rely on repeated attempts to return the expected results.
            result = subprocess.run(["incus", "exec", self.vm_name, "--", "journalctl", "-b", "-u", unit], capture_output=True, check=False)
            if regex:
                match = re.search(log, str(result.stdout))
                if match:
                    return match
            else:
                if log in str(result.stdout):
                    return None

            if time.time() - start > timeout:
                raise IncusOSException(f"timed out waiting for log entry '{log}' to appear", result.stdout.decode("utf-8").split("\n"))

            time.sleep(1)

    def LogDoesntContain(self, unit, log):
        """Assert that the log doesn't contain the specified entry."""

        result = subprocess.run(["incus", "exec", self.vm_name, "--", "journalctl", "-b", "-u", unit], capture_output=True, check=True)

        if log in str(result.stdout):
            raise IncusOSException(f"wasn't expecting log entry '{log}' to appear")

    def APIRequest(self, path, method="GET", body=None, content_type=None, return_raw_content=False):
        """Perform a HTTP REST API call, and return the result."""

        args = ["incus", "exec", self.vm_name, "--", "curl", "--unix-socket", "/run/incus-os/unix.socket", "http://localhost"+path, "-X", method]
        cmd_input = None

        if body is not None:
            if isinstance(body, str):
                args.append("-d")
                args.append(body)
            elif isinstance(body, bytes):
                args.append("--data-binary")
                args.append("@-")
                cmd_input = body

            else:
                raise Excpetion("can only send a string or bytes in the request body")

        if content_type is not None:
            args.append("-H")
            args.append("Content-Type: "+ content_type)

        result = subprocess.run(args, input=cmd_input, capture_output=True, check=True)

        if not return_raw_content:
            return json.loads(result.stdout)
        else:
            return result.stdout

    def RunCommand(self, *cmd, capture_output=True, check=True):
        """Run a given command within the VM."""
        return subprocess.run(["incus", "exec", self.vm_name, "--", *cmd], capture_output=capture_output, check=check)
