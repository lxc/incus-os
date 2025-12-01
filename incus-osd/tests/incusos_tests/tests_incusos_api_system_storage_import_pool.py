import os
import tempfile

from .incus_test_vm import IncusTestVM, IncusOSException, util

def TestIncusOSAPISystemStorageImportPool(install_image):
    test_name = "incusos-api-system-storage-import-pool"
    test_seed = {
        "install.json": """{"target":{"id":"scsi-0QEMU_QEMU_HARDDISK_incus_root"}}""",
    }

    test_image, incusos_version = util._prepare_test_image(install_image, test_seed)

    with tempfile.NamedTemporaryFile(dir=os.getcwd()) as disk_img:
        disk_img.truncate(50*1024*1024*1024)

        with IncusTestVM(test_name, test_image) as vm:
            vm.AddDevice("disk1", "disk", "source="+disk_img.name)

            vm.WaitSystemReady(incusos_version, source="/dev/sdc", target="/dev/sd(a|b)")

            # Can't import an unencrypted pool
            vm.RunCommand("zpool", "create", "mypool", "/dev/sdb")
            vm.RunCommand("zpool", "export", "mypool")

            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"mypool","type":"zfs","encryption_key":"NONE"}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success importing unencrypted pool")

            if result["error"] != "refusing to import unencrypted ZFS pool":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Can't import an encrypted pool that doesn't use a raw key
            vm.RunCommand("sgdisk", "-Z", "/dev/sdb")
            vm.RunCommand("zpool", "create", "-O", "encryption=aes-256-gcm", "-O", "keyformat=passphrase", "-O", "keylocation=file:///var/lib/incus-os/state.txt", "mypool", "/dev/sdb")
            vm.RunCommand("zpool", "export", "mypool")

            result = vm.APIRequest("/1.0/system/storage/:import-pool", method="POST", body="""{"name":"mypool","type":"zfs","encryption_key":"secret-passphrase"}""")
            if result["status_code"] == 200:
                raise IncusOSException("unexpected success importing encrypted pool with passphrase")

            if result["error"] != "refusing to import pool that doesn't use a raw encryption key":
                raise IncusOSException("unexpected error message: " + result["error"])

            # Can't import an encrypted pool with an incorrect key
            vm.RunCommand("sgdisk", "-Z", "/dev/sdb")
            vm.RunCommand("zpool", "create", "-O", "encryption=aes-256-gcm", "-O", "keyformat=raw", "-O", "keylocation=file:///var/lib/incus-os/zpool.local.key", "mypool", "/dev/sdb")
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
