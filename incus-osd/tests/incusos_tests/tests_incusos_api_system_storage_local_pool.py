import os
import tempfile

from .incus_test_vm import IncusTestVM, util

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
            raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

        if len(result["metadata"]["state"]["drives"]) != 1:
            raise Exception("expected exactly one drive")

        if len(result["metadata"]["state"]["pools"]) != 1:
            raise Exception("expected exactly one pool")

        if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "local":
            raise Exception("drive isn't part of the local pool")

        if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                raise Exception("expected one member device for local pool")

        if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
            raise Exception("pool doesn't have expected member device")

        if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
            raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

        if result["metadata"]["state"]["pools"][0]["name"] != "local":
            raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

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
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["drives"]) != 2:
                raise Exception("expected exactly two drives")

            if len(result["metadata"]["state"]["pools"]) != 1:
                raise Exception("expected exactly one pool")

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise Exception("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                raise Exception("first drive is part of a pool")

            if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                raise Exception("second drive isn't part of the local pool")

            if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                raise Exception("expected one member device for local pool")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise Exception("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

            # Extend the "local" pool with the second drive
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Get the updated storage state
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["drives"]) != 2:
                raise Exception("expected exactly two drives")

            if len(result["metadata"]["state"]["pools"]) != 1:
                raise Exception("expected exactly one pool")

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise Exception("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "local":
                raise Exception("first drive isn't part of the local pool")

            if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                raise Exception("second drive isn't part of the local pool")

            if len(result["metadata"]["state"]["pools"][0]["devices"]) != 2:
                raise Exception("expected two member devices for local pool")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise Exception("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["devices"][1] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise Exception("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

            # Don't allow removal of the main system partition from the pool
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1",""]}]}}""")
            if result["status_code"] == 200:
                raise Exception("unexpected success removing main storage partition")

            if result["error"] != "special zpool 'local' must always include main system partition '/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11'":
                raise Exception("unexpected error message: " + result["error"])

            # Remove the second drive from the pool
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid0","devices":["","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11"]}]}}""")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            # Get the updated storage state
            result = vm.APIRequest("/1.0/system/storage")
            if result["status_code"] != 200:
                raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            if len(result["metadata"]["state"]["drives"]) != 2:
                raise Exception("expected exactly two drives")

            if len(result["metadata"]["state"]["pools"]) != 1:
                raise Exception("expected exactly one pool")

            if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                raise Exception("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

            if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                raise Exception("first drive is part of a pool")

            if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

            if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                raise Exception("second drive isn't part of the local pool")

            if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                raise Exception("expected one member device for local pool")

            if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                raise Exception("pool doesn't have expected member device")

            if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

            if result["metadata"]["state"]["pools"][0]["name"] != "local":
                raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

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
                    raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["drives"]) != 3:
                    raise Exception("expected exactly three drives")

                if len(result["metadata"]["state"]["pools"]) != 1:
                    raise Exception("expected exactly one pool")

                if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                    raise Exception("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

                if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                    raise Exception("first drive is part of a pool")

                if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2":
                    raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

                if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "":
                    raise Exception("second drive is part of a pool")

                if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                    raise Exception("unexpected third drive: " + result["metadata"]["state"]["drives"][2]["id"])

                if result["metadata"]["state"]["drives"][2].get("member_pool", "") != "local":
                    raise Exception("third drive isn't part of the local pool")

                if len(result["metadata"]["state"]["pools"][0]["devices"]) != 1:
                    raise Exception("expected one member device for local pool")

                if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                    raise Exception("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid0":
                    raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

                if result["metadata"]["state"]["pools"][0]["name"] != "local":
                    raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

                # Extend the "local" pool with the second drive
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
                if result["status_code"] != 200:
                    raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                # Get the updated storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["drives"]) != 3:
                    raise Exception("expected exactly three drives")

                if len(result["metadata"]["state"]["pools"]) != 1:
                    raise Exception("expected exactly one pool")

                if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                    raise Exception("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

                if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "local":
                    raise Exception("first drive isn't part of the local pool")

                if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2":
                    raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

                if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "":
                    raise Exception("second drive is part of a pool")

                if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                    raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][2]["id"])

                if result["metadata"]["state"]["drives"][2].get("member_pool", "") != "local":
                    raise Exception("second drive isn't part of the local pool")

                if len(result["metadata"]["state"]["pools"][0]["devices"]) != 2:
                    raise Exception("expected two member devices for local pool")

                if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1-part11":
                    raise Exception("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["devices"][1] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                    raise Exception("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid1":
                    raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

                if result["metadata"]["state"]["pools"][0]["name"] != "local":
                    raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])

                # Can't add a third device to the "local" pool
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]}]}}""")
                if result["status_code"] == 200:
                    raise Exception("unexpected success adding third drive")

                if result["error"] != "special zpool 'local' cannot consist of more than two devices":
                    raise Exception("unexpected error message: " + result["error"])

                # Replace the second drive with the third
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"pools":[{"name":"local","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11"]}]}}""")
                if result["status_code"] != 200:
                    raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                # Get the updated storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise Exception("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["drives"]) != 3:
                    raise Exception("expected exactly three drives")

                if len(result["metadata"]["state"]["pools"]) != 1:
                    raise Exception("expected exactly one pool")

                if result["metadata"]["state"]["drives"][0]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1":
                    raise Exception("unexpected first drive: " + result["metadata"]["state"]["drives"][0]["id"])

                if result["metadata"]["state"]["drives"][0].get("member_pool", "") != "":
                    raise Exception("first drive is part of a pool")

                if result["metadata"]["state"]["drives"][1]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2":
                    raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][1]["id"])

                if result["metadata"]["state"]["drives"][1].get("member_pool", "") != "local":
                    raise Exception("second drive isn't part of the local pool")

                if result["metadata"]["state"]["drives"][2]["id"] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root":
                    raise Exception("unexpected second drive: " + result["metadata"]["state"]["drives"][2]["id"])

                if result["metadata"]["state"]["drives"][2].get("member_pool", "") != "local":
                    raise Exception("second drive isn't part of the local pool")

                if len(result["metadata"]["state"]["pools"][0]["devices"]) != 2:
                    raise Exception("expected two member devices for local pool")

                if result["metadata"]["state"]["pools"][0]["devices"][0] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2-part11":
                    raise Exception("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["devices"][1] != "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11":
                    raise Exception("pool doesn't have expected member device")

                if result["metadata"]["state"]["pools"][0]["type"] != "zfs-raid1":
                    raise Exception("pool has unexpected type: " + result["metadata"]["state"]["pools"][0]["type"])

                if result["metadata"]["state"]["pools"][0]["name"] != "local":
                    raise Exception("pool has unexpected name: " + result["metadata"]["state"]["pools"][0]["name"])
