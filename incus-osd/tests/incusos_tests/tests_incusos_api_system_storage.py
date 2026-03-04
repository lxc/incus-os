import json
import os
import tempfile
import time

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemStorageImportPool(install_image):
    test_name = "incusos-api-system-storage-import-pool"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(10*1024*1024*1024)

        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(incusos_version)

            # Can't import an unencrypted pool
            vm.RunCommand("zpool", "create", "mypool", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.RunCommand("zpool", "export", "mypool")

            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"mypool","type":"zfs","encryption_key":"NONE"}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success importing unencrypted pool")

            if result["error"] != "refusing to import unencrypted ZFS pool":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Can't import an encrypted pool that doesn't use a raw key
            vm.RunCommand("sgdisk", "-Z", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.RunCommand("zpool", "create", "-O", "encryption=aes-256-gcm", "-O", "keyformat=passphrase", "-O", "keylocation=file:///var/lib/incus-os/state.txt", "mypool", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.RunCommand("zpool", "export", "mypool")

            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"mypool","type":"zfs","encryption_key":"secret-passphrase"}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success importing encrypted pool with passphrase")

            if result["error"] != "refusing to import pool that doesn't use a raw encryption key":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Can't import an encrypted pool with an incorrect key
            vm.RunCommand("sgdisk", "-Z", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.RunCommand("zpool", "create", "-O", "encryption=aes-256-gcm", "-O", "keyformat=raw", "-O", "keylocation=file:///var/lib/incus-os/zpool.local.key", "mypool", "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1")
            vm.RunCommand("zpool", "export", "mypool")

            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"mypool","type":"zfs","encryption_key":"KoPGQLcHG/u4p8F82Jyl8mDfeElTEWlHE7pQV6bClCw="}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success importing encrypted pool with incorrect key")

            if result["error"] != "Failed to run: zfs load-key mypool: exit status 255 (Key load error: Incorrect key provided for 'mypool'.)":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Get the correct encryption key and verify a successful pool import
            result = vm.APIRequest("/1.0/system/security")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

            key = '"' + result["metadata"]["state"]["pool_recovery_keys"]["local"] + '"'

            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"mypool","type":"zfs","encryption_key":""" + key + "}")
            if result["status_code"] != 200:
                raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

def TestIncusOSAPISystemStorageMixedDeviceSize(install_image):
    test_name = "incusos-api-system-storage-mixed-device-size"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img1:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img2:
            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img3:
                disk_img1.truncate(10*1024*1024*1024)
                disk_img2.truncate(10*1024*1024*1024)
                disk_img3.truncate(11*1024*1024*1024)

                with IncusTestVM(test_name, test_image) as vm:
                    vm.AddDevice("disk1", "disk", "source="+disk_img1.name)
                    vm.AddDevice("disk2", "disk", "source="+disk_img2.name)
                    vm.AddDevice("disk3", "disk", "source="+disk_img3.name)

                    vm.WaitSystemReady(incusos_version)

                    # By default, can't create a storage pool with devices of different sizes.
                    result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                    if result["status_code"] == 200:
                        raise IncusOSException("unexpected success creating pool with mismatched devices")

                    if result["error"] != "refusing to create new zpool with devices of different sizes unless AllowMixedDevSizes is true":
                        raise IncusOSException("unexpected error message: " + result["error"])

                    # Set AllowMixedDevSizes to true and we should be able to create the storage pool.
                    result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raidz1","allow_mixed_dev_sizes":true,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    # Get the updated storage state.
                    result = vm.APIRequest("/1.0/system/storage")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    if len(result["metadata"]["state"]["pools"]) != 2:
                        raise IncusOSException("expected two storage pools")

                    poolState = result["metadata"]["state"]["pools"][0]
                    if poolState["name"] != "mypool":
                        poolState = result["metadata"]["state"]["pools"][1]

                    if len(poolState["devices"]) != 3:
                        raise IncusOSException("expected three member devices for 'mypool' pool")

def TestIncusOSAPISystemStorageConvertToMirror(install_image):
    test_name = "incusos-api-system-storage-convert-to-mirror"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img1:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img2:
            disk_img1.truncate(10*1024*1024*1024)
            disk_img2.truncate(10*1024*1024*1024)

            with IncusTestVM(test_name, test_image) as vm:
                vm.AddDevice("disk1", "disk", "source="+disk_img1.name)
                vm.AddDevice("disk2", "disk", "source="+disk_img2.name)

                vm.WaitSystemReady(incusos_version)

                # Create a storage pool using a single device.
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                # Get the current storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["pools"]) != 2:
                    raise IncusOSException("expected two storage pools")

                poolState = result["metadata"]["state"]["pools"][0]
                if poolState["name"] != "mypool":
                    poolState = result["metadata"]["state"]["pools"][1]

                if len(poolState["devices"]) != 1:
                    raise IncusOSException("expected exactly one device for 'mypool' pool")

                if poolState["type"] != "zfs-raid0":
                    raise IncusOSException("'mypool' type isn't zfs-raid0")

                # Convert the storage pool to a mirrored configuration.
                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]}]}}""")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                # Get the updated storage state.
                result = vm.APIRequest("/1.0/system/storage")
                if result["status_code"] != 200:
                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                if len(result["metadata"]["state"]["pools"]) != 2:
                    raise IncusOSException("expected two storage pools")

                poolState = result["metadata"]["state"]["pools"][0]
                if poolState["name"] != "mypool":
                    poolState = result["metadata"]["state"]["pools"][1]

                if len(poolState["devices"]) != 2:
                    raise IncusOSException("expected exactly two devices for 'mypool' pool")

                if poolState["type"] != "zfs-raid1":
                    raise IncusOSException("'mypool' type isn't zfs-raid1")

def TestIncusOSAPISystemStoragePoolLogDevices(install_image):
    test_name = "incusos-api-system-storage-pool-log-devices"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img1:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img2:
            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img3:
                disk_img1.truncate(10*1024*1024*1024)
                disk_img2.truncate(10*1024*1024*1024)
                disk_img3.truncate(10*1024*1024*1024)

                with IncusTestVM(test_name, test_image) as vm:
                    vm.AddDevice("disk1", "disk", "source="+disk_img1.name)
                    vm.AddDevice("disk2", "disk", "source="+disk_img2.name)
                    vm.AddDevice("disk3", "disk", "source="+disk_img3.name)

                    vm.WaitSystemReady(incusos_version)

                    # Create a simple pool with one log device.
                    result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"],"log":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]}]}}""")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    # Get the current storage state.
                    result = vm.APIRequest("/1.0/system/storage")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    if len(result["metadata"]["state"]["pools"]) != 2:
                        raise IncusOSException("expected two storage pools")

                    poolState = result["metadata"]["state"]["pools"][0]
                    if poolState["name"] != "mypool":
                        poolState = result["metadata"]["state"]["pools"][1]

                    if poolState["type"] != "zfs-raid0":
                        raise IncusOSException("'mypool' type isn't zfs-raid0")

                    if len(poolState["devices"]) != 1:
                        raise IncusOSException("expected exactly one device for 'mypool' pool")

                    if len(poolState["log"]) != 1:
                        raise IncusOSException("expected exactly one log device for 'mypool' pool")

                    # Introspect the zpool state and verify the single log device is a normal disk
                    result = vm.RunCommand("zpool", "status", "-jp", "--json-int")
                    poolState = json.loads(result.stdout)

                    if "scsi-0QEMU_QEMU_HARDDISK_incus_disk2" not in poolState["pools"]["mypool"]["logs"]:
                        raise IncusOSException("missing expected log device scsi-0QEMU_QEMU_HARDDISK_incus_disk2 for 'mypool' pool")

                    if poolState["pools"]["mypool"]["logs"]["scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]["vdev_type"] != "disk":
                        raise IncusOSException("log device scsi-0QEMU_QEMU_HARDDISK_incus_disk2 'mypool' pool isn't a disk")

                    # Delete the pool
                    result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    # Create a simple pool with two log devices.
                    result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"],"log":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    # Get the current storage state.
                    result = vm.APIRequest("/1.0/system/storage")
                    if result["status_code"] != 200:
                        raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                    if len(result["metadata"]["state"]["pools"]) != 2:
                        raise IncusOSException("expected two storage pools")

                    poolState = result["metadata"]["state"]["pools"][0]
                    if poolState["name"] != "mypool":
                        poolState = result["metadata"]["state"]["pools"][1]

                    if poolState["type"] != "zfs-raid0":
                        raise IncusOSException("'mypool' type isn't zfs-raid0")

                    if len(poolState["devices"]) != 1:
                        raise IncusOSException("expected exactly one device for 'mypool' pool")

                    if len(poolState["log"]) != 2:
                        raise IncusOSException("expected exactly two log devices for 'mypool' pool")

                    if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2" not in poolState["log"]:
                        raise IncusOSException("missing expected log device scsi-0QEMU_QEMU_HARDDISK_incus_disk2 for 'mypool' pool")

                    if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3" not in poolState["log"]:
                        raise IncusOSException("missing expected log device scsi-0QEMU_QEMU_HARDDISK_incus_disk3 for 'mypool' pool")

                    # Introspect the zpool state and verify the log device is a mirror of two disks
                    result = vm.RunCommand("zpool", "status", "-jp", "--json-int")
                    poolState = json.loads(result.stdout)

                    if "mirror-1" not in poolState["pools"]["mypool"]["logs"]:
                        raise IncusOSException("missing expected log device mirror-1 for 'mypool' pool")

                    if poolState["pools"]["mypool"]["logs"]["mirror-1"]["vdev_type"] != "mirror":
                        raise IncusOSException("log device mirror-1 'mypool' pool isn't a mirror")

def TestIncusOSAPISystemStoragePoolDeleteReplaceDevices(install_image):
    test_name = "incusos-api-system-storage-pool-delrep-devs"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img1:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img2:
            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img3:
                with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img4:
                    disk_img1.truncate(10*1024*1024*1024)
                    disk_img2.truncate(10*1024*1024*1024)
                    disk_img3.truncate(10*1024*1024*1024)
                    disk_img4.truncate(10*1024*1024*1024)

                    with IncusTestVM(test_name, test_image) as vm:
                        vm.AddDevice("disk1", "disk", "source="+disk_img1.name)
                        vm.AddDevice("disk2", "disk", "source="+disk_img2.name)
                        vm.AddDevice("disk3", "disk", "source="+disk_img3.name)
                        vm.AddDevice("disk4", "disk", "source="+disk_img4.name)

                        vm.WaitSystemReady(incusos_version)

                        # Create a basic raidz1 pool
                        result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        # Get the current storage state.
                        result = vm.APIRequest("/1.0/system/storage")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        if len(result["metadata"]["state"]["pools"]) != 2:
                            raise IncusOSException("expected two storage pools")

                        poolState = result["metadata"]["state"]["pools"][0]
                        if poolState["name"] != "mypool":
                            poolState = result["metadata"]["state"]["pools"][1]

                        if poolState["type"] != "zfs-raid1":
                            raise IncusOSException("'mypool' type isn't zfs-raid1")

                        if poolState["state"] != "ONLINE":
                            raise IncusOSException("'mypool' state isn't ONLINE")

                        if len(poolState["devices"]) != 3:
                            raise IncusOSException("expected exactly three devices for 'mypool' pool")

                        # Delete (offline) one device in the pool
                        result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        # Sleep a second to allow resilver to finish
                        time.sleep(1)

                        # Get the updated storage state.
                        result = vm.APIRequest("/1.0/system/storage")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        if len(result["metadata"]["state"]["pools"]) != 2:
                            raise IncusOSException("expected two storage pools")

                        poolState = result["metadata"]["state"]["pools"][0]
                        if poolState["name"] != "mypool":
                            poolState = result["metadata"]["state"]["pools"][1]

                        if poolState["type"] != "zfs-raid1":
                            raise IncusOSException("'mypool' type isn't zfs-raid1")

                        if poolState["state"] != "DEGRADED":
                            raise IncusOSException("'mypool' state isn't DEGRADED")

                        if len(poolState["devices"]) != 2:
                            raise IncusOSException("expected exactly two devices for 'mypool' pool")

                        if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1" in poolState["devices"]:
                            raise IncusOSException("removed device in 'mypool' pool incorrectly reported as a member")

                        # Replace (online) the pool's removed device
                        result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        # Sleep a second to allow resilver to finish
                        time.sleep(1)

                        # Get the updated storage state.
                        result = vm.APIRequest("/1.0/system/storage")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        if len(result["metadata"]["state"]["pools"]) != 2:
                            raise IncusOSException("expected two storage pools")

                        poolState = result["metadata"]["state"]["pools"][0]
                        if poolState["name"] != "mypool":
                            poolState = result["metadata"]["state"]["pools"][1]

                        if poolState["type"] != "zfs-raid1":
                            raise IncusOSException("'mypool' type isn't zfs-raid1")

                        if poolState["state"] != "ONLINE":
                            raise IncusOSException("'mypool' state isn't ONLINE")

                        if len(poolState["devices"]) != 3:
                            raise IncusOSException("expected exactly three devices for 'mypool' pool")

                        # Replace one device with another in the pool
                        result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4"]}]}}""")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        # Sleep a second to allow resilver to finish
                        time.sleep(1)

                        # Get the updated storage state.
                        result = vm.APIRequest("/1.0/system/storage")
                        if result["status_code"] != 200:
                            raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                        if len(result["metadata"]["state"]["pools"]) != 2:
                            raise IncusOSException("expected two storage pools")

                        poolState = result["metadata"]["state"]["pools"][0]
                        if poolState["name"] != "mypool":
                            poolState = result["metadata"]["state"]["pools"][1]

                        if poolState["type"] != "zfs-raid1":
                            raise IncusOSException("'mypool' type isn't zfs-raid1")

                        if poolState["state"] != "ONLINE":
                            raise IncusOSException("'mypool' state isn't ONLINE")

                        if len(poolState["devices"]) != 3:
                            raise IncusOSException("expected exactly three devices for 'mypool' pool")

                        if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3" in poolState["devices"]:
                            raise IncusOSException("removed device in 'mypool' pool incorrectly reported as a member")

                        if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4" not in poolState["devices"]:
                            raise IncusOSException("new device in 'mypool' pool not reported as a member")

def TestIncusOSAPISystemStoragePoolExpansionTests(install_image):
    test_name = "incusos-api-system-storage-pool-expansion-tests"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img1:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img2:
            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img3:
                with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img4:
                    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img5:
                        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img6:
                            disk_img1.truncate(10*1024*1024*1024)
                            disk_img2.truncate(10*1024*1024*1024)
                            disk_img3.truncate(10*1024*1024*1024)
                            disk_img4.truncate(10*1024*1024*1024)
                            disk_img5.truncate(10*1024*1024*1024)
                            disk_img6.truncate(10*1024*1024*1024)

                            with IncusTestVM(test_name, test_image) as vm:
                                vm.AddDevice("disk1", "disk", "source="+disk_img1.name)
                                vm.AddDevice("disk2", "disk", "source="+disk_img2.name)
                                vm.AddDevice("disk3", "disk", "source="+disk_img3.name)
                                vm.AddDevice("disk4", "disk", "source="+disk_img4.name)
                                vm.AddDevice("disk5", "disk", "source="+disk_img5.name)
                                vm.AddDevice("disk6", "disk", "source="+disk_img6.name)

                                vm.WaitSystemReady(incusos_version)

                                # raid0 testing

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid0":
                                    raise IncusOSException("'mypool' type isn't zfs-raid0")

                                if len(poolState["devices"]) != 1:
                                    raise IncusOSException("expected exactly one device for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 10200547328:
                                    raise IncusOSException("'mypool' size 10200547328 != " + str(poolState[1]["usable_pool_size_in_bytes"]))

                                # Expand the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the updated storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid0":
                                    raise IncusOSException("'mypool' type isn't zfs-raid0")

                                if len(poolState["devices"]) != 4:
                                    raise IncusOSException("expected exactly four devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 40802189312:
                                    raise IncusOSException("'mypool' size 40802189312 != " + str(poolState["usable_pool_size_in_bytes"]))

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # raid1 testing

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid1":
                                    raise IncusOSException("'mypool' type isn't zfs-raid1")

                                if len(poolState["devices"]) != 2:
                                    raise IncusOSException("expected exactly two devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 10200547328:
                                    raise IncusOSException("'mypool' size 10200547328 != " + str(poolState[1]["usable_pool_size_in_bytes"]))

                                # Expand the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the updated storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid1":
                                    raise IncusOSException("'mypool' type isn't zfs-raid1")

                                if len(poolState["devices"]) != 3:
                                    raise IncusOSException("expected exactly three devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 10200547328:
                                    raise IncusOSException("'mypool' size 10200547328 != " + str(poolState["usable_pool_size_in_bytes"]))

                                # Can't expand multiple devices at once
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5"]}]}}""")
                                if result["status_code"] == 200:
                                    raise IncusOSException("unexpected success expanding raid1 pool with multiple devices")

                                if result["error"] != "expanding a pool can only be performed one device at a time, due to a required resilver between expansions":
                                    raise IncusOSException("unexpected error message: " + result["error"])

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # raid10 testing

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid10","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid10":
                                    raise IncusOSException("'mypool' type isn't zfs-raid10")

                                if len(poolState["devices"]) != 4:
                                    raise IncusOSException("expected exactly four devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 20401094656:
                                    raise IncusOSException("'mypool' size 20401094656 != " + str(poolState[1]["usable_pool_size_in_bytes"]))

                                # Can't expand with only one device
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid10","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5"]}]}}""")
                                if result["status_code"] == 200:
                                    raise IncusOSException("unexpected success expanding raid10 pool with a single device")

                                if result["error"] != "expanding a raid10 pool requires a pair of new devices":
                                    raise IncusOSException("unexpected error message: " + result["error"])

                                # Expand the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raid10","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk6"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the updated storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid10":
                                    raise IncusOSException("'mypool' type isn't zfs-raid10")

                                if len(poolState["devices"]) != 6:
                                    raise IncusOSException("expected exactly six devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 20401094656:
                                    raise IncusOSException("'mypool' size 20401094656 != " + str(poolState["usable_pool_size_in_bytes"]))

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # raidz testing

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raidz1":
                                    raise IncusOSException("'mypool' type isn't zfs-raidz1")

                                if len(poolState["devices"]) != 3:
                                    raise IncusOSException("expected exactly three devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 21096300544:
                                    raise IncusOSException("'mypool' size 21096300544 != " + str(poolState[1]["usable_pool_size_in_bytes"]))

                                # Expand the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4"]}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Sleep a second to allow resilver to finish
                                time.sleep(1)

                                # Get the updated storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raidz1":
                                    raise IncusOSException("'mypool' type isn't zfs-raidz1")

                                if len(poolState["devices"]) != 4:
                                    raise IncusOSException("expected exactly four devices for 'mypool' pool")

                                if poolState["usable_pool_size_in_bytes"] != 28247588864:
                                    raise IncusOSException("'mypool' size 28247588864 != " + str(poolState["usable_pool_size_in_bytes"]))

                                # Can't expand multiple devices at once
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule": "0 4 * * 0", "pools":[{"name":"mypool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk6"]}]}}""")
                                if result["status_code"] == 200:
                                    raise IncusOSException("unexpected success expanding raidz1 pool with multiple devices")

                                if result["error"] != "expanding a pool can only be performed one device at a time, due to a required resilver between expansions":
                                    raise IncusOSException("unexpected error message: " + result["error"])

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

def TestIncusOSAPISystemStoragePoolSpecialDevice(install_image):
    test_name = "incusos-api-system-storage-pool-special-device"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img1:
        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img2:
            with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img3:
                with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img4:
                    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img5:
                        with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img6:
                            disk_img1.truncate(10*1024*1024*1024)
                            disk_img2.truncate(10*1024*1024*1024)
                            disk_img3.truncate(10*1024*1024*1024)
                            disk_img4.truncate(10*1024*1024*1024)
                            disk_img5.truncate(10*1024*1024*1024)
                            disk_img6.truncate(10*1024*1024*1024)

                            with IncusTestVM(test_name, test_image) as vm:
                                vm.AddDevice("disk1", "disk", "source="+disk_img1.name)
                                vm.AddDevice("disk2", "disk", "source="+disk_img2.name)
                                vm.AddDevice("disk3", "disk", "source="+disk_img3.name)
                                vm.AddDevice("disk4", "disk", "source="+disk_img4.name)
                                vm.AddDevice("disk5", "disk", "source="+disk_img5.name)
                                vm.AddDevice("disk6", "disk", "source="+disk_img6.name)

                                vm.WaitSystemReady(incusos_version)

                                # Special device for a raid0 pool

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule":"0 4 * * 0","pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"],"special":{"type":"zfs-raid0","special_small_blocks_size_in_kb":16,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"]}}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid0":
                                    raise IncusOSException("'mypool' type isn't zfs-raid0")

                                if len(poolState["devices"]) != 1:
                                    raise IncusOSException("expected exactly one device for 'mypool' pool")

                                if poolState["special"]["type"] != "zfs-raid0":
                                    raise IncusOSException("'mypool' special dev type isn't zfs-raid0")

                                if poolState["special"]["special_small_blocks_size_in_kb"] != 16:
                                    raise IncusOSException("'mypool' special dev small_blocks_size isn't 16: " + str(poolState["special"]["special_small_blocks_size_in_kb"]))

                                if len(poolState["special"]["devices"]) != 1:
                                    raise IncusOSException("expected exactly one member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk2 isn't a member device for 'mypool' pool special dev")

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule":"0 4 * * 0","pools":[{"name":"mypool","type":"zfs-raid0","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"],"special":{"type":"zfs-raid0","special_small_blocks_size_in_kb":24,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"]}}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid0":
                                    raise IncusOSException("'mypool' type isn't zfs-raid0")

                                if len(poolState["devices"]) != 1:
                                    raise IncusOSException("expected exactly one device for 'mypool' pool")

                                if poolState["special"]["type"] != "zfs-raid0":
                                    raise IncusOSException("'mypool' special dev type isn't zfs-raid0")

                                if poolState["special"]["special_small_blocks_size_in_kb"] != 24:
                                    raise IncusOSException("'mypool' special dev small_blocks_size isn't 24: " + str(poolState["special"]["special_small_blocks_size_in_kb"]))

                                if len(poolState["special"]["devices"]) != 2:
                                    raise IncusOSException("expected exactly two member devices for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk2 isn't a member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk3 isn't a member device for 'mypool' pool special dev")

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Special device for a raid1 pool

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule":"0 4 * * 0","pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"],"special":{"type":"zfs-raid1","special_small_blocks_size_in_kb":32,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4"]}}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid1":
                                    raise IncusOSException("'mypool' type isn't zfs-raid1")

                                if len(poolState["devices"]) != 2:
                                    raise IncusOSException("expected exactly two devices for 'mypool' pool")

                                if poolState["special"]["type"] != "zfs-raid1":
                                    raise IncusOSException("'mypool' special dev type isn't zfs-raid1")

                                if poolState["special"]["special_small_blocks_size_in_kb"] != 32:
                                    raise IncusOSException("'mypool' special dev small_blocks_size isn't 32: " + str(poolState["special"]["special_small_blocks_size_in_kb"]))

                                if len(poolState["special"]["devices"]) != 2:
                                    raise IncusOSException("expected exactly two member devices for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk3 isn't a member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk4 isn't a member device for 'mypool' pool special dev")

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule":"0 4 * * 0","pools":[{"name":"mypool","type":"zfs-raid1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2"],"special":{"type":"zfs-raidz1","special_small_blocks_size_in_kb":48,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4"]}}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raid1":
                                    raise IncusOSException("'mypool' type isn't zfs-raid1")

                                if len(poolState["devices"]) != 2:
                                    raise IncusOSException("expected exactly two devices for 'mypool' pool")

                                if poolState["special"]["type"] != "zfs-raidz1":
                                    raise IncusOSException("'mypool' special dev type isn't zfs-raidz1")

                                if poolState["special"]["special_small_blocks_size_in_kb"] != 48:
                                    raise IncusOSException("'mypool' special dev small_blocks_size isn't 48: " + str(poolState["special"]["special_small_blocks_size_in_kb"]))

                                if len(poolState["special"]["devices"]) != 2:
                                    raise IncusOSException("expected exactly two member devices for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk3 isn't a member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk4 isn't a member device for 'mypool' pool special dev")

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Special device for a raidz1 pool

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule":"0 4 * * 0","pools":[{"name":"mypool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"],"special":{"type":"zfs-raid1","special_small_blocks_size_in_kb":64,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5"]}}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raidz1":
                                    raise IncusOSException("'mypool' type isn't zfs-raidz1")

                                if len(poolState["devices"]) != 3:
                                    raise IncusOSException("expected exactly three devices for 'mypool' pool")

                                if poolState["special"]["type"] != "zfs-raid1":
                                    raise IncusOSException("'mypool' special dev type isn't zfs-raid1")

                                if poolState["special"]["special_small_blocks_size_in_kb"] != 64:
                                    raise IncusOSException("'mypool' special dev small_blocks_size isn't 64: " + str(poolState["special"]["special_small_blocks_size_in_kb"]))

                                if len(poolState["special"]["devices"]) != 2:
                                    raise IncusOSException("expected exactly two member devices for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk4 isn't a member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk5 isn't a member device for 'mypool' pool special dev")

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Create the pool
                                result = vm.APIRequest("/1.0/system/storage", method="PUT", body="""{"config":{"scrub_schedule":"0 4 * * 0","pools":[{"name":"mypool","type":"zfs-raidz1","devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk3"],"special":{"type":"zfs-raidz1","special_small_blocks_size_in_kb":128,"devices":["/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5","/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk6"]}}]}}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                # Get the current storage state.
                                result = vm.APIRequest("/1.0/system/storage")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))

                                if len(result["metadata"]["state"]["pools"]) != 2:
                                    raise IncusOSException("expected two storage pools")

                                poolState = result["metadata"]["state"]["pools"][0]
                                if poolState["name"] != "mypool":
                                    poolState = result["metadata"]["state"]["pools"][1]

                                if poolState["type"] != "zfs-raidz1":
                                    raise IncusOSException("'mypool' type isn't zfs-raidz1")

                                if len(poolState["devices"]) != 3:
                                    raise IncusOSException("expected exactly three devices for 'mypool' pool")

                                if poolState["special"]["type"] != "zfs-raidz1":
                                    raise IncusOSException("'mypool' special dev type isn't zfs-raidz1")

                                if poolState["special"]["special_small_blocks_size_in_kb"] != 128:
                                    raise IncusOSException("'mypool' special dev small_blocks_size isn't 128: " + str(poolState["special"]["special_small_blocks_size_in_kb"]))

                                if len(poolState["special"]["devices"]) != 3:
                                    raise IncusOSException("expected exactly three member devices for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk4" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk4 isn't a member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk5" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk5 isn't a member device for 'mypool' pool special dev")

                                if "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk6" not in poolState["special"]["devices"]:
                                    raise IncusOSException("scsi-0QEMU_QEMU_HARDDISK_incus_disk6 isn't a member device for 'mypool' pool special dev")

                                # Delete the pool
                                result = vm.APIRequest("/1.0/system/storage/:delete-pool", method="POST", body="""{"name":"mypool"}""")
                                if result["status_code"] != 200:
                                    raise IncusOSException("unexpected status code %d: %s" % (result["status_code"], result["error"]))
