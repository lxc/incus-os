import os
import tempfile

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestInstallMultipath(install_image):
    test_name = "multipath"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-3500577a4300c1e34"}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        # Truncate the disk image file to 50GiB.
        disk_img.truncate(50*1024*1024*1024)

        with IncusTestVM(test_name, test_image) as vm:
            _commonMultipathChecks(vm, incusos_version, disk_img.name)

            # Shouldn't see any mention of a degraded security state
            vm.LogDoesntContain("incus-osd", "Degraded security state:")

def TestInstallMultipathUseSWTPM(install_image):
    test_name = "multipath-use-swtpm"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-3500577a4300c1e34"},"security":{"missing_tpm":true}}"""
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        # Truncate the disk image file to 50GiB.
        disk_img.truncate(50*1024*1024*1024)

        with IncusTestVM(test_name, test_image) as vm:
            vm.RemoveDevice("vtpm")

            _commonMultipathChecks(vm, incusos_version, disk_img.name)

            # Should see a log message about swtpm
            vm.WaitExpectedLog("incus-osd", "Degraded security state: no physical TPM found, using swtpm")

def _commonMultipathChecks(vm, incusos_version, disk):
    vm.AddDevice("multipathdisk1", "disk", "source="+disk, "wwn=0x500577a4300c1e34")
    vm.AddDevice("multipathdisk2", "disk", "source="+disk, "wwn=0x500577a4300c1e34")

    vm.WaitSystemReady(incusos_version, target="/dev/disk/by-id/scsi-3500577a4300c1e34")

    # Verify that two of the drives belong to the multipath device. Because symlinks have been
    # overwritten, we can't just check specific drives under /dev/disk/by-id/.
    num_multipath_devices = 0
    for device in ["sda", "sdb", "sdc"]:
        result = vm.RunCommand("multipath", "-c", "/dev/"+device, check=False)
        if """DM_MULTIPATH_DEVICE_PATH="1""" in str(result):
            num_multipath_devices += 1

    if num_multipath_devices != 2:
        raise IncusOSException("Only found " + str(num_multipath_devices) + " of 2 expected multipath device members")

    # Verify that multipath-backed boot device is properly up.
    result = vm.RunCommand("multipath", "-ll")
    if "3500577a4300c1e34 dm-0 QEMU,QEMU HARDDISK" not in str(result):
        raise IncusOSException("Multipath device '3500577a4300c1e34' not reported as active")

    result = vm.RunCommand("lsblk", "/dev/mapper/3500577a4300c1e34")
    if "3500577a4300c1e34          252:0    0    50G  0 mpath" not in str(result):
        raise IncusOSException("Multipath device isn't properly reported")
    if "root                   252:13   0    25G  0 crypt /" not in str(result):
        raise IncusOSException("Root partition not properly reported")
