# Storage

IncusOS allows for the configuration of complex ZFS storage pools. Each pool is automatically encrypted with a randomly generated key to protect data stored in the pool. The encryption keys can be retrieved from the system's security state.

When creating a storage pool, IncusOS can use local devices, or remote devices made available via a [service](../services.md), such as iSCSI.

It is also possible to add, remove, and replace devices from an existing storage pool. This is accomplished by getting the current pool configuration, making the necessary changes in the relevant struct, then submitting the results back to IncusOS.

```{note}
Unencrypted ZFS storage pools are not supported. IncusOS will only create encrypted pools, and will refuse to import any existing unencrypted pool.

This prevents the accidental leakage of sensitive data from an encrypted pool to an unencrypted one.
```

## Configuration options

The following configuration options can be set:

* `pools`: An array of zero or more user-defined storage pool definitions.

### Examples

Create a storage pool `mypool` as ZFS raidz1 with four devices, one cache device, and one log device:

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

Get the pool encryption keys for safe storage (base64 encoded):

```
$ incus admin os system security show
[snip]
state:
  pool_recovery_keys:
    local: vIAKUWSxK5GrNrkn60kjEXh2M4WZdtX+hcyhx0W8q7U=
    mypool: zh9gkAgGsKenO48y7dwNg6aBFaD6OoedgSlSsivEq0Q=
```

## Deleting a storage pool

```{warning}
Deleting a storage pool will result in the unrecoverable loss of all data in that pool.
```

Delete the storage pool `mypool` by running

```
incus admin os system storage delete-pool -d '{"name":"mypool"}'
```

## Wiping a drive

```{warning}
Wiping a drive will result in the unrecoverable loss of all data on that drive.
```

Wipe drive `scsi-0QEMU_QEMU_HARDDISK_incus_disk`, which must be specified by its ID, by running

```
./incus admin os system storage wipe-drive -d '{"id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk"}'
```

## Importing an existing encrypted pool

If importing an existing storage pool, IncusOS needs to be informed of its encryption key before the data can be made available. Because there is no way to prompt for an encryption passphrase, only ZFS pools using a raw encryption key can be imported. Specify the raw base64 encoded encryption key when importing storage pool `mypool` by running

```
incus admin os system storage import-storage-pool -d '{"name":"mypool","type":"zfs","encryption_key":"THp6YZ33zwAEXiCWU71/l7tY8uWouKB5TSr/uKXCj2A="}'
```

## Managing volumes

It's possible to create and delete volumes within a storage pool.

Each volume has its own:

* Name
* Quota (in bytes, a zero value means unrestricted)
* Use (`incus` or `linstor`)

The list of volumes are visible directly in the storage state data.

Creating and deleting volumes can be done through the command line with:

```
incus admin os system storage create-volume -d '{"pool":"local","name":"my-volume","use":"linstor"}'
incus admin os system storage delete-volume -d '{"pool":"local","name":"my-volume"}'
```

```{note}
IncusOS automatically creates a new `incus` volume when setting up the `local` storage pool.
```
