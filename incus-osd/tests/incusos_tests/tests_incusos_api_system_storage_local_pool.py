import os
import tempfile
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemStorageLocalPool(install_image):
    test_name = "incusos-api-system-storage-local-pool"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Get current storage state.
        result = vm.APIRequest("/1.0/system/storage")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["state"]["drives"]) != 1:
            raise IncusOSException("expected exactly one drive")

        if len(result["metadata"]["state"]["pools"]) != 1:
            raise IncusOSException("expected exactly one pool")

        if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "local":
            raise IncusOSException("drive isn't part of the local pool")

        if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                raise IncusOSException("expected one member device for local pool")

        if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
            raise IncusOSException("pool doesn't have expected member device")

        if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
            raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

        if result["metadata"]["state"]["pools"][0]["name"] != "local":
            raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

def TestIncusOSAPISystemStorageLocalPoolExpandRAID0(install_image):
    test_name = "incusos-api-system-storage-local-pool-expand-raid0"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(50*1024*1024*1024)

        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(incusos_version, source="/dev/sdc")

            # Get current storage state.
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["drives"]) != 2:
                raise IncusOSException("expected exactly two drives")

            if len(result["metadata"]["state"]["pools"]) != 1:
                raise IncusOSException("expected exactly one pool")

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise IncusOSException("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                raise IncusOSException("first drive is part of a pool")

            if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                raise IncusOSException("second drive isn't part of the local pool")

            if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                raise IncusOSException("expected one member device for local pool")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise IncusOSException("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

            # Extend the "local" pool with the second drive
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Get the updated storage state
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["drives"]) != 2:
                raise IncusOSException("expected exactly two drives")

            if len(result["metadata"]["state"]["pools"]) != 1:
                raise IncusOSException("expected exactly one pool")

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise IncusOSException("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "local":
                raise IncusOSException("first drive isn't part of the local pool")

            if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                raise IncusOSException("second drive isn't part of the local pool")

            if len(result["metadata"]["state"]["pools"][0]["devices"]) != 2:
                raise IncusOSException("expected two member devices for local pool")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise IncusOSException("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["devices"][1] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise IncusOSException("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

            # Don't allow removal of the main system partition from the pool
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1",""]}]}}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success removing main storage partition")

            if result["error"] != "special zpool 'local' must always include main system partition '/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11'":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Remove the second drive from the pool
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid0","devices":["","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11"]}]}}""")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Get the updated storage state
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["drives"]) != 2:
                raise IncusOSException("expected exactly two drives")

            if len(result["metadata"]["state"]["pools"]) != 1:
                raise IncusOSException("expected exactly one pool")

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise IncusOSException("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                raise IncusOSException("first drive is part of a pool")

            if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                raise IncusOSException("second drive isn't part of the local pool")

            if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                raise IncusOSException("expected one member device for local pool")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise IncusOSException("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

def TestIncusOSAPISystemStorageLocalPoolExpandRAID1(install_image):
    test_name = "incusos-api-system-storage-local-pool-expand-raid1"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk1_img:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk2_img:
            disk1_img.truncate(50*1024*1024*1024)
            disk2_img.truncate(50*1024*1024*1024)

            with IncusTestVM(test_name, test_image) as vm:
                vm.AddDevice("disk1", "disk", "source="+disk1_img.name)
                vm.AddDevice("disk2", "disk", "source="+disk2_img.name)

                vm.WaitSystemReady(incusos_version, source="/dev/sdd")

                # Get current storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["drives"]) != 3:
                    raise IncusOSException("expected exactly three drives")

                if len(result["metadata"]["state"]["pools"]) != 1:
                    raise IncusOSException("expected exactly one pool")

                if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                    raise IncusOSException("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

                if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                    raise IncusOSException("first drive is part of a pool")

                if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2":
                    raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

                if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "":
                    raise IncusOSException("second drive is part of a pool")

                if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                    raise IncusOSException("unexpected third drive: " + result["metadata"]["state"]["drives"][2]["id"])

                if result["metadata"]["state"]["drives"][2].get("member_pool", "") != "local":
                    raise IncusOSException("third drive isn't part of the local pool")

                if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                    raise IncusOSException("expected one member device for local pool")

                if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                    raise IncusOSException("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                    raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

                if result["metadata"]["state"]["pools"][0]["name"] != "local":
                    raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

                # Extend the "local" pool with the second drive
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                # Get the updated storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["drives"]) != 3:
                    raise IncusOSException("expected exactly three drives")

                if len(result["metadata"]["state"]["pools"]) != 1:
                    raise IncusOSException("expected exactly one pool")

                if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                    raise IncusOSException("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

                if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "local":
                    raise IncusOSException("first drive isn't part of the local pool")

                if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2":
                    raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

                if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "":
                    raise IncusOSException("second drive is part of a pool")

                if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                    raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][2]["id"])

                if result["metadata"]["state"]["drives"][2].get("member_pool", "") != "local":
                    raise IncusOSException("second drive isn't part of the local pool")

                if len(result["metadata"]["state"]["pools"][0]["devices"]) != 2:
                    raise IncusOSException("expected two member devices for local pool")

                if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1-part11":
                    raise IncusOSException("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["devices"][1] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                    raise IncusOSException("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid1":
                    raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

                if result["metadata"]["state"]["pools"][0]["name"] != "local":
                    raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

                # Can't add a third device to the "local" pool
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]}]}}""")
                if result["status_code"] == 200:
                    raise IncusOSException("unexpected success adding third drive")

                if result["error"] != "special zpool 'local' cannot consist of more than two devices":
                    raise IncusOSException("unexpected error message: " + result["error"])

                # Replace the second drive with the third
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11"]}]}}""")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                # Get the updated storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["drives"]) != 3:
                    raise IncusOSException("expected exactly three drives")

                if len(result["metadata"]["state"]["pools"]) != 1:
                    raise IncusOSException("expected exactly one pool")

                if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                    raise IncusOSException("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

                if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                    raise IncusOSException("first drive is part of a pool")

                if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2":
                    raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

                if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                    raise IncusOSException("second drive isn't part of the local pool")

                if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                    raise IncusOSException("unexpected second drive: " + result["metadata"]["state"]["drives"][2]["id"])

                if result["metadata"]["state"]["drives"][2].get("member_pool", "") != "local":
                    raise IncusOSException("second drive isn't part of the local pool")

                if len(result["metadata"]["state"]["pools"][0]["devices"]) != 2:
                    raise IncusOSException("expected two member devices for local pool")

                if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2-part11":
                    raise IncusOSException("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["devices"][1] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                    raise IncusOSException("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid1":
                    raise IncusOSException("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

                if result["metadata"]["state"]["pools"][0]["name"] != "local":
                    raise IncusOSException("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

def TestIncusOSAPISystemStorageLocalPoolRecoverFreshInstall(install_image):
    test_name = "incusos-api-system-storage-local-pool-recover"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(50*1024*1024*1024)

        encryption_key = ""

        test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

        # First, configure an existing "local" pool
        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(incusos_version, source="/dev/sdc")

            # Convert "local" pool to RAID1 and get its encryption key
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            result = vm.APIRequest("/1.0/system/security")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            encryption_key = result["metadata"]["state"]["pool_recovery_keys"]["local"]

        # Second, install a new VM and recover the existing "local" pool
        test_image, incusos_version = util._prepare_test_image(install_image, test_seed)
        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(incusos_version, source="/dev/sdc")
            vm.WaitExpectedLog("incus-osd", "Attempting to recover storage pool 'local' using existing non-system drive")

            # After the pool is recovered, re-import it via API
            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"local","type":"zfs","encryption_key":""" + '"' + encryption_key + '"}')
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))


def TestIncusOSAPISystemStorageLocalPoolScrub(install_image):
    test_name = "incusos-api-system-storage-local-pool-scrub"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with IncusTestVM(test_name, test_image) as vm:
        vm.WaitSystemReady(incusos_version)

        # Get current storage state.
        result = vm.APIRequest("/1.0/system/storage")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["state"]["pools"]) != 1:
            raise IncusOSException("expected exactly one pool")

        if "last_scrub" in result["metadata"]["state"]["pools"][0]:
            raise IncusOSException("expected no last_scrub to be reported since to scrub was requested")

        # Scrub the pool.
        result = vm.APIRequest("/1.0/system/storage/:scrub-pool", method="POST", body="""{"name":"local"}""")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        # Wait for scrub to complete.
        time.sleep(5)

        # Get current storage state.
        result = vm.APIRequest("/1.0/system/storage")
        if result["status_code"] != 200:
            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["state"]["pools"]) != 1:
            raise IncusOSException("expected exactly one pool")

        if "last_scrub" not in result["metadata"]["state"]["pools"][0]:
            raise IncusOSException("expected last_scrub to be reported after scrubbing the pool")

        if "start_time" not in result["metadata"]["state"]["pools"][0]["last_scrub"]:
            raise IncusOSException("expected start time to be reported on the last scrub")

        if "end_time" not in result["metadata"]["state"]["pools"][0]["last_scrub"]:
            raise IncusOSException("expected end time to be reported on the last scrub")

        if result["metadata"]["state"]["pools"][0]["last_scrub"]["state"] != "FINISHED":
            raise IncusOSException("expected last scrub to have 'FINISHED' status")

        if result["metadata"]["state"]["pools"][0]["last_scrub"]["progress"] != "100.00%":
            raise IncusOSException("expected progress to be reported on the last scrub")

        if result["metadata"]["state"]["pools"][0]["last_scrub"]["errors"] != 0:
            raise IncusOSException("expected 0 errors to be reported on the last scrub")
