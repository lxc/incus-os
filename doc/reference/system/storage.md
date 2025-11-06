# Storage

IncusOS allows for the configuration of complex local ZFS storage pools. Each pool is automatically encrypted with a randomly generated key to protect data stored in the pool. The encryption keys can be retrieved from the system's security state.

It is also possible to add, remove, and replace devices from an existing local storage pool. This is accomplished by getting the current pool configuration, making the necessary changes in the relevant struct, then submitting the results back to IncusOS.

## Configuration options

The following configuration options can be set:

* `pools`: An array of zero or more user-defined local storage pool definitions.

### Examples

Create a local storage pool `mypool` as ZFS raidz1 with four devices, one cache device, and one log device:

```
{
    "pools": [
        {"name":"mypool",
         "type":"zfs-raidz1",
         "devices":["/dev/sdb","/dev/sdc","/dev/sdd","/dev/sde"],
         "cache":["/dev/sdf"],
         "log":["/dev/sdg"]}
    ]
}
```

Replace failed device `/dev/sdb` with `/dev/sdh`:

```
{
    "pools": [
        {"name":"mypool",
         "type":"zfs-raidz1",
         "devices":["/dev/sdh","/dev/sdc","/dev/sdd","/dev/sde"],
         "cache":["/dev/sdf"],
         "log":["/dev/sdg"]}
    ]
}
```

Get the local pool encryption keys for safe storage (base64 encoded):

```
$ incus admin os system show security
[snip]
state:
  pool_recovery_keys:
    local: vIAKUWSxK5GrNrkn60kjEXh2M4WZdtX+hcyhx0W8q7U=
    mypool: zh9gkAgGsKenO48y7dwNg6aBFaD6OoedgSlSsivEq0Q=
```

## Deleting a local storage pool

```{warning}
Deleting a storage pool will result in the unrecoverable loss of all data in that pool.
```

Delete the local storage pool `mypool` by running

```
incus admin os system delete-storage-pool -d '{"name":"mypool"}'
```

## Wiping a local drive

```{warning}
Wiping a drive will result in the unrecoverable loss of all data on that drive.
```

Wipe local drive `scsi-0QEMU_QEMU_HARDDISK_incus_disk`, which must be specified by its ID, by running

```
./incus admin os system wipe-drive -d '{"id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk"}'
```

## Importing encryption key for an existing local pool

If importing an existing local storage pool, IncusOS needs to be informed of its encryption key before the data can be made available. Import the raw base64 encoded encryption key for storage pool `mypool` by running

```
incus admin os system import-storage-encryption-key -d '{"name":"mypool","type":"zfs","encryption_key":"THp6YZ33zwAEXiCWU71/l7tY8uWouKB5TSr/uKXCj2A="}'
```
