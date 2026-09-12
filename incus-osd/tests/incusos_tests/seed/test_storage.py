from ..incus_test_vm import IncusTestVM, IncusOSException, util

import os
import tempfile

def TestSeedStorageExtraPool(install_image):
    test_name = "seed-storage-extra-pool"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
        "storage.json": """{"pools":[{"name":"mypool","alignment":512,"type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}""",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(10*1024*1024*1024)

        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(os_version)

            # Check that the pools have their expected alignment
            result = vm.APIRequest("/1.0/system/storage", method="GET")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            if len(result["metadata"]["config"]["pools"]) != 2:
                raise IncusOSException("expected two storage pools")

            localPool = result["metadata"]["config"]["pools"][0]
            myPool = result["metadata"]["config"]["pools"][1]
            if localPool["name"] != "local":
                localPool = result["metadata"]["config"]["pools"][1]
                myPool = result["metadata"]["config"]["pools"][0]

            if localPool["alignment"] != 4096:
                raise IncusOSException("local pool alignment isn't 4096: " + str(localPool["alignment"]))

            if myPool["alignment"] != 512:
                raise IncusOSException("mypool pool alignment isn't 512: " + str(myPool["alignment"]))

            # Delete mypool and verify we can't create a pool with an invalid alignment
            result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "trim_schedule": "0 4 * * 6", "pools":[{"name":"mypool","type":"zfs-raid0","alignment":1023,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success creating pool with invalid alignment")

            if result["error"] != "pool alignment value must be a power of two":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Create a new pool with an alignment of 1024
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "trim_schedule": "0 4 * * 6", "pools":[{"name":"mypool","type":"zfs-raid0","alignment":1024,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            # Verify the new pool has the expected alignment
            result = vm.APIRequest("/1.0/system/storage", method="GET")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            if len(result["metadata"]["config"]["pools"]) != 2:
                raise IncusOSException("expected two storage pools")

            myPool = result["metadata"]["config"]["pools"][0]
            if myPool["name"] != "mypool":
                myPool = result["metadata"]["config"]["pools"][1]

            if myPool["alignment"] != 1024:
                raise IncusOSException("mypool pool alignment isn't 1024: " + str(myPool["alignment"]))

            # Can't change the alignment for an existing pool
            result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "trim_schedule": "0 4 * * 6", "pools":[{"name":"mypool","type":"zfs-raid0","alignment":512,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success changing pool alignment")

            if result["error"] != "cannot change pool alignment after creation":
                raise IncusOSException("unexpected error message: " + result["error"])

def TestSeedStorageLocalPool(install_image):
    test_name = "seed-storage-local-pool"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
        "storage.json": """{"pools":[{"name":"local","alignment":2048,"allow_mixed_dev_sizes":true,"type":"zfs-raid1","devices":["/dev/disk/by-partlabel/local-data","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}""",
    }

    test_image, os_name, os_version, client_cert_name = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(50*1024*1024*1024)

        with IncusTestVM(os_name, test_name, test_image, client_cert_name) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(os_version)

            # Check that the local pool is properly setup
            result = vm.APIRequest("/1.0/system/storage", method="GET")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["error_code"], result["error"]))

            if len(result["metadata"]["config"]["pools"]) != 1:
                raise IncusOSException("expected exactly one storage pool")

            localPool = result["metadata"]["config"]["pools"][0]

            if localPool["alignment"] != 2048:
                raise IncusOSException("local pool alignment isn't 2048: " + str(localPool["alignment"]))

            if localPool["type"] != "zfs-raid1":
                raise IncusOSException("local pool type isn't zfs-raid1: " + localPool["type"])

            if len(localPool["devices"]) != 2:
                raise IncusOSException("expected local pool to consist of exactly two devices")
