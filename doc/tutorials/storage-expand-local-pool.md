# Expanding the "local" storage pool

On first boot, IncusOS will automatically create a storage pool called "local" on the main system drive. This pool will consume all available free space remaining on the drive. (Further details about the partitioning of the main system drive are available [here](../reference/partitioning-scheme.md).)

A common scenario is installing IncusOS on a server with two drives, each being the same size. IncusOS will install on one, which leaves an unused drive. Rather than creating a separate storage pool on the unused drive, we can extend the automatically created "local" storage pool using the second drive. It is possible to configure the second drive as RAID0 (striping) or RAID1 (mirror).

## Limitations

Some limitations apply to the "local" pool:

* The main system drive partition cannot be removed from the "local" pool
* The "local" pool cannot be deleted
* Only RAID0 and RAID1 are supported for the "local" pool
* The "local" pool can consist of exactly one or two drives

### Additional RAID1 limitations

* The second drive must be the same size as the main system drive
* The pool capacity will be ~35GiB less than the size of the drives due to partitioning layout on the main system drive

## Initial system state

For this tutorial, let's assume there are three drives, each 50GiB in size. IncusOS is installed, but otherwise no changes have been made to the system.

We can get the system's current storage state:

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
  scrub_schedule: 0 4 * * 0
state:
  drives:
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk1
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk2
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
    available_in_bytes: 2.5271922688e+10
    size_in_bytes: 2.6225119232e+10
```

## RAID0

RAID0 maximizes available space for the "local" pool at the expense of no data redundancy when a drive fails.

Let's add `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1` to the "local" pool by running `incus admin os system storage edit`. The configuration should look like:

```
config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      name: local
      type: zfs-raid0
  scrub_schedule: 0 4 * * 0
```

After saving and exiting, IncusOS will apply the changes. We can see the updated storage configuration reflecting the expansion of the "local" storage pool:

```
$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      encryption_key_status: available
      managed: true
      name: local
      pool_allocated_space_in_bytes: 4.882432e+06
      raw_pool_size_in_bytes: 7.0866960384e+10
      state: ONLINE
      type: zfs-raid0
      usable_pool_size_in_bytes: 7.0866960384e+10
      volumes:
        - name: incus
          quota_in_bytes: 0
          usage_in_bytes: 2.965504e+06
          use: incus
  scrub_schedule: 0 4 * * 0
state:
  drives:
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      member_pool: local
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk1
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk2
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
    available_in_bytes: 2.5271922688e+10
    size_in_bytes: 2.6225119232e+10
```

## RAID1

RAID1 mirrors data written to the "local" pool which allows for recovery of data if one drive fails.

Let's add `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1` to the "local" pool by running `incus admin os system storage edit`. The configuration should look like:

```
config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      name: local
      type: zfs-raid1
  scrub_schedule: 0 4 * * 0
```

```{note}
Note that in addition to adding the second drive, the type is also changed to `zfs-raid1`.
```

After saving and exiting, IncusOS will apply the changes. We can see the updated storage configuration reflecting the expansion of the "local" storage pool:

```

$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1-part11
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      encryption_key_status: available
      last_scrub:
        end_time: "2026-08-27T19:45:13Z"
        errors: 0
        progress: 100.00%
        start_time: "2026-08-27T19:45:13Z"
        state: FINISHED
      managed: true
      name: local
      pool_allocated_space_in_bytes: 4.58752e+06
      raw_pool_size_in_bytes: 1.7716740096e+10
      state: ONLINE
      type: zfs-raid1
      usable_pool_size_in_bytes: 1.7716740096e+10
      volumes:
        - name: incus
          quota_in_bytes: 0
          usage_in_bytes: 2.965504e+06
          use: incus
  scrub_schedule: 0 4 * * 0
state:
  drives:
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      member_pool: local
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk1
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk2
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
    available_in_bytes: 2.5271922688e+10
    size_in_bytes: 2.6225119232e+10
```

### Recovering a failed non-system drive

Let's pretend the second drive we added to the "local" storage pool is dying. We happen to have a third drive available in the server which we can use to replace the failing one.

Once again, run `incus admin os system storage edit` and replace `disk1` with `disk2`:

```
config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      name: local
      type: zfs-raid1
  scrub_schedule: 0 4 * * 0
```

```{note}
When specifying devices for a pool, order is important. IncusOS will always return a sorted list which it will use when comparing the list of devices it receives via the API to determine what device(s) to add, remove, or replace in the pool. Put another way, `"devices": ["/dev/sda", "/dev/sdb"]` != `"devices": ["/dev/sdb", "/dev/sda"]`.
```

After saving and exiting, IncusOS will apply the changes. Depending on how much data is stored in the "local" pool, it might take some time for ZFS to finish the resilver. Eventually the process will complete, and the system's storage state will look like the following:

```
$ incus admin os system storage show
WARNING: The IncusOS API and configuration is subject to change

config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2-part11
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      encryption_key_status: available
      last_scrub:
        end_time: "2026-08-27T19:47:36Z"
        errors: 0
        progress: 100.00%
        start_time: "2026-08-27T19:47:36Z"
        state: FINISHED
      managed: true
      name: local
      pool_allocated_space_in_bytes: 4.734976e+06
      raw_pool_size_in_bytes: 1.7716740096e+10
      state: ONLINE
      type: zfs-raid1
      usable_pool_size_in_bytes: 1.7716740096e+10
      volumes:
        - name: incus
          quota_in_bytes: 0
          usage_in_bytes: 2.965504e+06
          use: incus
  scrub_schedule: 0 4 * * 0
state:
  drives:
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk1
    - boot: false
      bus: scsi
      capacity_in_bytes: 5.36870912e+10
      id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
      member_pool: local
      model_family: QEMU
      model_name: QEMU HARDDISK
      multipath: false
      remote: false
      removable: false
      serial_number: incus_disk2
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
    available_in_bytes: 2.5271922688e+10
    size_in_bytes: 2.6225119232e+10
```

### Recovering a failed system drive

If the main system drive fails, it is possible to recover the data from the "local" storage pool. After reinstalling IncusOS on a new drive, if the second drive is physically present on first boot IncusOS will attempt to recover the "local" storage pool:

```
2026-08-27 19:52:00 INFO Bringing up the local storage
2026-08-27 19:52:01 INFO Attempting to recover storage pool 'local' using existing non-system drive
[snip]
2026-08-27 19:52:25 INFO System is ready version=202608271552
```

This will restore the pool to a good state, but because this is a fresh IncusOS install you must supply the encryption key for the previously-created "local" storage pool:

```
incus admin os system storage import-storage-pool -d '{"name":"local","type":"zfs","encryption_key":"QWJKYnRLGfyhj+OevRfgkdE6MW6PgAqR57tTi+8T+qA="}'
```

After this step, the data in the "local" pool will now be available and automatically unencrypted on each boot.

Depending on what application(s) are installed, additional steps may be required to fully restore the newly reinstalled system. For example, if Incus is installed, you might need to run`incus admin recover` to re-discover the instances stored in the "local" pool.
