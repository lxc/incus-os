# Import existing Incus instances from an unencrypted pool

Scenario: An existing server is running Incus with an unencrypted ZFS storage pool configured for its instances. We would like to install IncusOS and then migrate the existing instances to an encrypted storage volume that will be managed by IncusOS.

Prerequisites:

* An existing unencrypted ZFS storage pool used by Incus
* One or more drives to create a new ZFS storage pool

## Existing Incus setup

We'll assume the existing ZFS storage pool is called `oldpool` throughout this tutorial. We don't care too much about the pool's configuration, except that it is unencrypted:

```
bash-5.2# zfs get encryption oldpool
NAME     PROPERTY    VALUE        SOURCE
oldpool  encryption  off          default
```

Incus has configured this ZFS pool as a storage pool called `incus`:

```
$ incus storage list
┌───────┬────────┬──────────────────────────────────────┬─────────┬─────────┐
│ NAME  │ DRIVER │             DESCRIPTION              │ USED BY │  STATE  │
├───────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ incus │ zfs    │                                      │ 4       │ CREATED │
├───────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ local │ zfs    │ Local storage pool (on system drive) │ 4       │ CREATED │
└───────┴────────┴──────────────────────────────────────┴─────────┴─────────┘
```

There are two instances on the server:

```
$ incus list
┌─────────────┬─────────┬───────────────────────┬──────────────────────────────────────────────────┬─────────────────┬───────────┐
│    NAME     │  STATE  │         IPV4          │                       IPV6                       │      TYPE       │ SNAPSHOTS │
├─────────────┼─────────┼───────────────────────┼──────────────────────────────────────────────────┼─────────────────┼───────────┤
│ debian13    │ RUNNING │ 10.48.100.68 (eth0)   │ fd42:8ee2:bc1a:484b:1266:6aff:fe7f:88f7 (eth0)   │ CONTAINER       │ 0         │
├─────────────┼─────────┼───────────────────────┼──────────────────────────────────────────────────┼─────────────────┼───────────┤
│ debian13-vm │ RUNNING │ 10.48.100.75 (enp5s0) │ fd42:8ee2:bc1a:484b:1266:6aff:fec3:b763 (enp5s0) │ VIRTUAL-MACHINE │ 0         │
└─────────────┴─────────┴───────────────────────┴──────────────────────────────────────────────────┴─────────────────┴───────────┘
```

Ensure all instances are stopped before powering down the system to install IncusOS:

```
for instance in $(incus list --columns n --format compact,noheader); do incus stop $instance; done
```

## Install IncusOS and create a new encrypted storage volume

Follow the [instructions to install IncusOS](../getting-started/installation.md) on the server.

Once IncusOS is installed, we will create a new ZFS pool `newpool` via the IncusOS API. In this tutorial, for simplicity it will consist of a single drive, but more complex/robust pool configuration is possible.

`oldpool` exists on `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1` and `newpool` will be created on `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2`.

Show the system's current storage state:

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
      pool_allocated_space_in_bytes: 4.575232e+06
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

Create `newpool`:

```
incus admin os system storage edit
```

```
config:
  pools:
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
      name: local
      type: zfs-raid0
    - devices:
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
      name: newpool
      type: zfs-raid0
  scrub_schedule: 0 4 * * 0
```

Create a storage volume for Incus to use in `newpool`:

```
incus admin os system storage create-volume -d '{"pool":"newpool","name":"incus","use":"incus"}'
```

Show that the new ZFS pool and volume have indeed been created:

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
      pool_allocated_space_in_bytes: 4.575232e+06
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
        - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
      encryption_key_status: available
      managed: true
      name: newpool
      pool_allocated_space_in_bytes: 1.101824e+06
      raw_pool_size_in_bytes: 5.3150220288e+10
      state: ONLINE
      type: zfs-raid0
      usable_pool_size_in_bytes: 5.3150220288e+10
      volumes:
        - name: incus
          quota_in_bytes: 0
          usage_in_bytes: 196608
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
      member_pool: newpool
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
    available_in_bytes: 2.5271918592e+10
    size_in_bytes: 2.6225119232e+10
```

## Create a new Incus storage pool

Next, create a new Incus storage pool called `incus_new` using the new storage volume:

```
$ incus storage create incus_new zfs source=newpool/incus
Storage pool incus_new created
$ incus storage list
┌───────────┬────────┬──────────────────────────────────────┬─────────┬─────────┐
│   NAME    │ DRIVER │             DESCRIPTION              │ USED BY │  STATE  │
├───────────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ incus_new │ zfs    │                                      │ 0       │ CREATED │
├───────────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ local     │ zfs    │ Local storage pool (on system drive) │ 4       │ CREATED │
└───────────┴────────┴──────────────────────────────────────┴─────────┴─────────┘
```

## Use `incus admin recover` to import existing instances

```
$ incus admin recover
This server currently has the following storage pools:
 - incus_new (backend="zfs", source="newpool/incus")
 - local (backend="zfs", source="local/incus")
Would you like to recover another storage pool? (yes/no) [default=no]: yes
Name of the storage pool: incus
Name of the storage backend (btrfs, dir, lvm, lvmcluster, zfs): zfs
Source of the storage pool (block device, volume group, dataset, path, ... as applicable): oldpool
Additional storage pool configuration property (KEY=VALUE, empty when done):
Would you like to recover another storage pool? (yes/no) [default=no]:
The recovery process will be scanning the following storage pools:
 - EXISTING: "incus_new" (backend="zfs", source="newpool/incus")
 - EXISTING: "local" (backend="zfs", source="local/incus")
 - NEW: "incus" (backend="zfs", source="oldpool")
Would you like to continue with scanning for lost volumes? (yes/no) [default=yes]:
Scanning for unknown volumes...
The following unknown storage pools have been found:
 - Storage pool "incus" of type "zfs"
The following unknown volumes have been found:
 - Container "debian13" on pool "incus" in project "default" (includes 0 snapshots)
 - Virtual-Machine "debian13-vm" on pool "incus" in project "default" (includes 0 snapshots)
Would you like those to be recovered? (yes/no) [default=no]: yes
Starting recovery...
```

## Move existing instances to new storage volume

Now that we have both the old and new ZFS pools available, we can move the instances from the unencrypted `oldpool` to the encrypted `newpool`:

```
for instance in $(incus list --columns n --format compact,noheader); do incus move $instance $instance --storage incus_new; done
```

Once complete, delete the old Incus storage pool:

```
$ incus storage delete incus
Storage pool incus deleted
$ incus storage list
┌───────────┬────────┬──────────────────────────────────────┬─────────┬─────────┐
│   NAME    │ DRIVER │             DESCRIPTION              │ USED BY │  STATE  │
├───────────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ incus_new │ zfs    │                                      │ 2       │ CREATED │
├───────────┼────────┼──────────────────────────────────────┼─────────┼─────────┤
│ local     │ zfs    │ Local storage pool (on system drive) │ 4       │ CREATED │
└───────────┴────────┴──────────────────────────────────────┴─────────┴─────────┘
```

## Start instances and verify running

Now you can start the migrated instances on the IncusOS server using the encrypted storage volume:

```
$ for instance in $(incus list --columns n --format compact,noheader); do incus start $instance; done
$ incus list
┌─────────────┬─────────┬───────────────────────┬──────────────────────────────────────────────────┬─────────────────┬───────────┐
│    NAME     │  STATE  │         IPV4          │                       IPV6                       │      TYPE       │ SNAPSHOTS │
├─────────────┼─────────┼───────────────────────┼──────────────────────────────────────────────────┼─────────────────┼───────────┤
│ debian13    │ RUNNING │ 10.203.93.68 (eth0)   │ fd42:39f9:1631:1319:1266:6aff:fe7f:88f7 (eth0)   │ CONTAINER       │ 0         │
├─────────────┼─────────┼───────────────────────┼──────────────────────────────────────────────────┼─────────────────┼───────────┤
│ debian13-vm │ RUNNING │ 10.203.93.75 (enp5s0) │ fd42:39f9:1631:1319:1266:6aff:fec3:b763 (enp5s0) │ VIRTUAL-MACHINE │ 0         │
└─────────────┴─────────┴───────────────────────┴──────────────────────────────────────────────────┴─────────────────┴───────────┘

```

## Wipe old disk(s)

Finally, you can wipe the disk(s) that composed the old, unencrypted storage pool:

```
$ incus admin os system storage wipe-drive -d '{"id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"}'
WARNING: The IncusOS API and configuration is subject to change

Are you sure you want to wipe the drive? (yes/no) [default=no]: yes
```

Once complete they can be used to create another pool or extend an existing one. Or you can physically remove them from the IncusOS server.
