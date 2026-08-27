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
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      name: local
      type: zfs-raid0
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      name: my-pool
      type: zfs-raid0
  scrub_schedule: 0 4 * * 0
```

Afterwards we see the new pool is created, but unused:

```
$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      encryption_key_status: available
      managed: true
      name: local
      pool_allocated_space_in_bytes: 4.562944e+06
      raw_pool_size_in_bytes: 1.7716740096e+10
      state: ONLINE
      type: zfs-raid0
      usable_pool_size_in_bytes: 1.7716740096e+10
      volumes:
        - name: incus
          quota_in_bytes: 0
          usage_in_bytes: 2.965504e+06
          use: incus
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      encryption_key_status: available
      managed: true
      name: my-pool
      pool_allocated_space_in_bytes: 540672
      raw_pool_size_in_bytes: 5.3150220288e+10
      state: ONLINE
      type: zfs-raid0
      usable_pool_size_in_bytes: 5.3150220288e+10
      volumes: []
  scrub_schedule: 0 4 * * 0
state:
  drives:
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      member_pool: my-pool
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk1
    - boot: true
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root
      member_pool: local
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_root
  root_partition:
    available_in_bytes: 2.5271918592e+10
    size_in_bytes: 2.6225119232e+10
```

## Creating a volume

We will now create a new storage volume `my-volume` for use by Incus:

```
$ incus admin os system storage create-volume -d '{"pool":"my-pool","name":"my-volume","use":"incus"}'
$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      encryption_key_status: available
      managed: true
      name: local
      pool_allocated_space_in_bytes: 4.562944e+06
      raw_pool_size_in_bytes: 1.7716740096e+10
      state: ONLINE
      type: zfs-raid0
      usable_pool_size_in_bytes: 1.7716740096e+10
      volumes:
        - name: incus
          quota_in_bytes: 0
          usage_in_bytes: 2.965504e+06
          use: incus
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      encryption_key_status: available
      managed: true
      name: my-pool
      pool_allocated_space_in_bytes: 1.101824e+06
      raw_pool_size_in_bytes: 5.3150220288e+10
      state: ONLINE
      type: zfs-raid0
      usable_pool_size_in_bytes: 5.3150220288e+10
      volumes:
        - name: my-volume
          quota_in_bytes: 0
          usage_in_bytes: 196608
          use: incus

[snip]
```

## Making the volume available to Incus

Finally, we can easily add the storage volume for Incus to use:

```
$ incus storage create incusos-volume zfs source=my-pool/my-volume
Storage pool incusos-volume created
$ incus storage list
┌────────────────┬────────┬──────────────────────────────────────┬─────────┬─────────┐
│      NAME      │ DRIVER │             DESCRIPTION              │ USED BY │  STATE  │
├────────────────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ incusos-volume │ zfs    │                                      │ 0       │ CREATED │
├────────────────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ local          │ zfs    │ Local storage pool (on system drive) │ 4       │ CREATED │
└────────────────┴────────┴──────────────────────────────────────┴─────────┴─────────┘
```
