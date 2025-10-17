import os
import tempfile
import time

from . import IncusTestVM, util

def TestIncusOSAPISystemStorage(install_image):
    test_name = "incusos-api-system-storage"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    # Setup six additional disks, each 10GiB in size: two SATA, two USB, and two NVME.
    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk1_img, tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk2_img, \
        tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk3_img, tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk4_img, \
        tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk5_img, tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk6_img:

        disk1_img.truncate(10*1024*1024*1024)
        disk2_img.truncate(10*1024*1024*1024)
        disk3_img.truncate(10*1024*1024*1024)
        disk4_img.truncate(10*1024*1024*1024)
        disk5_img.truncate(10*1024*1024*1024)
        disk6_img.truncate(10*1024*1024*1024)

        pool_encryption_key = ""

        # Create an initial VM where we will test most of the storage API.
        test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk1_img.name)
            vm.AddDevice("disk2", "disk", "source="+disk2_img.name)
            vm.AddDevice("disk3", "disk", "source="+disk3_img.name, "io.bus=usb")
            vm.AddDevice("disk4", "disk", "source="+disk4_img.name, "io.bus=usb")
            vm.AddDevice("disk5", "disk", "source="+disk5_img.name, "io.bus=nvme")
            vm.AddDevice("disk6", "disk", "source="+disk6_img.name, "io.bus=nvme")

            vm.WaitSystemReady(incusos_version, source="/dev/sd.")

            # Get current storage configuration and state.
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Should have seven drives.
            if len(result["metadata"]["state"]["drives"]) != 7:
                raise Exception("expected exactly seven drives to be present")

            if result["metadata"]["state"]["drives"][4]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise Exception("unexpected root drive: "+ result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise Exception("unexpected disk1 drive: "+ result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][2]["bus"] != "scsi":
                raise Exception("unexpected disk1 bus: "+ result["metadata"]["state"]["drives"][1]["bus"])

            if result["metadata"]["state"]["drives"][5]["id"] != "/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk3-0:0":
                raise Exception("unexpected disk3 drive: "+ result["metadata"]["state"]["drives"][3]["id"])

            if result["metadata"]["state"]["drives"][5]["bus"] != "":
                raise Exception("unexpected disk3 bus: "+ result["metadata"]["state"]["drives"][3]["bus"])

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk5":
                raise Exception("unexpected disk5 drive: "+ result["metadata"]["state"]["drives"][5]["id"])

            if result["metadata"]["state"]["drives"][0]["bus"] != "nvme":
                raise Exception("unexpected disk5 bus: "+ result["metadata"]["state"]["drives"][5]["bus"])

            # Should have one pool.
            if len(result["metadata"]["state"]["pools"]) != 1:
                raise Exception("expected exactly one pool to be present")

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise Exception("got unexpected pool name: '" + result["metadata"]["state"]["pools"][0]["name"] + "'")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise Exception("got unexpected pool type: '" + result["metadata"]["state"]["pools"][0]["type"] + "'")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise Exception("got unexpected pool device: '" + result["metadata"]["state"]["pools"][0]["devices"][0] + "'")

            if result["metadata"]["state"]["pools"][0]["encryption_key_status"] != "available":
                raise Exception("got unexpected pool key availability: '" + result["metadata"]["state"]["pools"][0]["encryption_key_status"] + "'")

            # Create a new storage pool with three member devices, one cache device and one log device.
            pool_config = """{"config":{"pools":[{"name":"testpool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk3-0:0","/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk5"],"cache":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"],"log":["/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk4-0:0"]}]}}"""
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body=pool_config)
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Get update pool status.
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Should have two pools now.
            if len(result["metadata"]["state"]["pools"]) != 2:
                raise Exception("expected exactly two pools to be present")

            # Check the newly created pool.
            for i in [0,2,3,5,6]:
                if result["metadata"]["state"]["drives"][i]["member_pool"] != "testpool":
                    raise Exception("%s isn't part of testpool" % result["metadata"]["state"]["drives"][i]["id"])

            if result["metadata"]["state"]["pools"][1]["name"] != "testpool":
                raise Exception("got unexpected test pool name: '" + result["metadata"]["state"]["pools"][1]["name"] + "'")

            if result["metadata"]["state"]["pools"][1]["type"] != "zfs-raidz1":
                raise Exception("got unexpected test pool type: '" + result["metadata"]["state"]["pools"][1]["type"] + "'")

            if result["metadata"]["state"]["pools"][1]["encryption_key_status"] != "available":
                raise Exception("got unexpected test pool key availability: '" + result["metadata"]["state"]["pools"][1]["encryption_key_status"] + "'")

            if len(result["metadata"]["state"]["pools"][1]["devices"]) != 3:
                raise Exception("expected exactly three devices for test pool, got %d" % len(result["metadata"]["state"]["pools"][1]["devices"]))

            if len(result["metadata"]["state"]["pools"][1]["cache"]) != 1:
                raise Exception("expected exactly one cache for test pool, got %d" % len(result["metadata"]["state"]["pools"][1]["cache"]))

            if len(result["metadata"]["state"]["pools"][1]["log"]) != 1:
                raise Exception("expected exactly one log for test pool, got %d" % len(result["metadata"]["state"]["pools"][1]["log"]))

            # Test swapping a drive in the storage pool.
            if result["metadata"]["state"]["pools"][1]["devices"][0] != "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk5":
                raise Exception("unexpected first device for test pool: '" + result["metadata"]["state"]["pools"][1]["devices"][0] + "'")

            pool_config = """{"config":{"pools":[{"name":"testpool","type":"zfs-raidz1","devices":["/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk6","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk3-0:0"],"cache":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"],"log":["/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk4-0:0"]}]}}"""
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body=pool_config)
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Sleep a few seconds to let ZFS do its magic.
            time.sleep(5)

            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["pools"][1]["devices"]) != 3:
                raise Exception("expected exactly three devices for test pool, got %d" % len(result["metadata"]["state"]["pools"][1]["devices"]))

            if result["metadata"]["state"]["pools"][1]["devices"][0] != "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk6":
                raise Exception("unexpected first device for test pool after swap: '" + result["metadata"]["state"]["pools"][1]["devices"][0] + "'")

            # Trying to add a "used" drive should fail.
            vm.RunCommand("sgdisk", "-n", "1", "/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk5")

            pool_config = """{"config":{"pools":[{"name":"testpool","type":"zfs-raidz1","devices":["/dev/disk/by-id/nvme-QEMU_NVMe_Ctrl_incus_disk5","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk3-0:0"],"cache":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"],"log":["/dev/disk/by-id/usb-QEMU_QEMU_HARDDISK_incus_disk4-0:0"]}]}}"""
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body=pool_config)
            if result["status_code"] != 0 or "is in use and contains a unknown filesystem" not in result["error"]:
                print(result)

                raise Exception("expected addition of a 'used' drive to fail, but it didn't")

            # Get the pool's encryption key to test re-importing.
            result = vm.APIRequest("/1.0/system/security")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            pool_encryption_key = result["metadata"]["state"]["PoolRecoveryKeys"]["testpool"]

        print(pool_encryption_key)

        return

        # Create a new VM and re-attach the test disks, then test importing an existing pool.
        test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk1_img.name)
            vm.AddDevice("disk2", "disk", "source="+disk2_img.name)
            vm.AddDevice("disk3", "disk", "source="+disk3_img.name, "io.bus=usb")
            vm.AddDevice("disk4", "disk", "source="+disk4_img.name, "io.bus=usb")
            vm.AddDevice("disk5", "disk", "source="+disk5_img.name, "io.bus=nvme")
            vm.AddDevice("disk6", "disk", "source="+disk6_img.name, "io.bus=nvme")

            vm.WaitSystemReady(incusos_version, source="/dev/sd.")
