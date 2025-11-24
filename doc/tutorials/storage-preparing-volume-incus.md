# Preparing a storage volume for Incus

IncusOS creates encrypted storage pools with randomly-generated encryption keys. These keys are stored on the main system drive that is in turn protected by TPM-backed encryption. This provides strong protection for all data stored in each storage pool.

Each storage pool can contain one or more volumes, which can be used to enforce storage quotas or be specially-configured for a specific application.

It's very easy to create a storage volume and make it available to an application, such as Incus.

```{warning}
Incus can [directly create a storage pool](https://linuxcontainers.org/incus/docs/main/howto/storage_pools/). However, this pool will be **unencrypted** and not managed by IncusOS. Because of this, it is strongly recommended to create a storage volume using the IncusOS API, then expose it to Incus as described below.
```

## Creating the storage pool

The [storage pool API](../reference/system/storage.md) provides options for creating complex pools. This tutorial will use a single drive for simplicity.

Assuming we want to create a pool `my-pool` using the device `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1`, run `incus admin os system storage edit` and add the following pool configuration:

```
config:
  pools:
    - name: my-pool
      type: zfs-raid0
      devices:
      - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
```

Afterwards we see the new pool is created, but unused:

```
gibmat@futurfusion:~$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

[snip]

state:
  pools:
  - devices:
    - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
    encryption_key_status: available
    name: my-pool
    pool_allocated_space_in_bytes: 724992
    raw_pool_size_in_bytes: 5.3150220288e+10
    state: ONLINE
    type: zfs-raid0
    usable_pool_size_in_bytes: 5.3150220288e+10
    volumes: []
```

## Creating a volume

We will now create a new storage volume `my-volume` for use by Incus:

```
gibmat@futurfusion:~$ incus admin os system storage create-volume -d '{"pool":"my-pool","name":"my-volume","use":"incus"}'
gibmat@futurfusion:~$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

[snip]

state:
  pools:
  - devices:
    - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
    encryption_key_status: available
    name: my-pool
    pool_allocated_space_in_bytes: 1.04448e+06
    raw_pool_size_in_bytes: 5.3150220288e+10
    state: ONLINE
    type: zfs-raid0
    usable_pool_size_in_bytes: 5.3150220288e+10
    volumes:
    - name: my-volume
      quota_in_bytes: 0
      usage_in_bytes: 196608
      use: incus
```

## Making the volume available to Incus

Finally, we can easily add the storage volume for Incus to use:

```
gibmat@futurfusion:~$ incus storage create incusos-volume zfs source=my-pool/my-volume
Storage pool incusos-volume created
gibmat@futurfusion:~$ incus storage list
+----------------+--------+--------------------------------------+---------+---------+
|      NAME      | DRIVER |             DESCRIPTION              | USED BY |  STATE  |
+----------------+--------+--------------------------------------+---------+---------+
| local          | zfs    | Local storage pool (on system drive) | 3       | CREATED |
+----------------+--------+--------------------------------------+---------+---------+
| incusos-volume | zfs    |                                      | 0       | CREATED |
+----------------+--------+--------------------------------------+---------+---------+
```
