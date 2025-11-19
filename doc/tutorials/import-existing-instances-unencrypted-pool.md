# Import existing Incus instances from an unencrypted local pool

Scenario: An existing server is running Incus with an unencrypted ZFS storage pool configured for its instances. We would like to install IncusOS and then migrate the existing instances to an encrypted storage pool.

Prerequisites:
* An existing unencrypted ZFS storage pool
* One or more drives to create a new ZFS storage pool

## Existing Incus setup

We'll assume the existing ZFS storage pool is called `oldpool` throughout this tutorial. We don't care too much about the pool's configuration, except that it is unencrypted:

```
bash-5.2# zfs get encryption oldpool
NAME     PROPERTY    VALUE        SOURCE
oldpool  encryption  off          default
```

Incus has configured this ZFS pool as a storage pool called "incus":

```
gibmat@futurfusion:~$ incus storage list
+-------+--------+--------------------------------------+---------+---------+
| NAME  | DRIVER |             DESCRIPTION              | USED BY |  STATE  |
+-------+--------+--------------------------------------+---------+---------+
| incus | zfs    |                                      | 4       | CREATED |
+-------+--------+--------------------------------------+---------+---------+
| local | zfs    | Local storage pool (on system drive) | 3       | CREATED |
+-------+--------+--------------------------------------+---------+---------+
```

There are two instances on the server:

```
gibmat@futurfusion:~$ incus list
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
|    NAME     |  STATE  |          IPV4           |                       IPV6                       |      TYPE       | SNAPSHOTS |
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
| debian13    | RUNNING | 10.104.236.121 (eth0)   | fd42:7786:2217:a97c:1266:6aff:fe30:bb19 (eth0)   | CONTAINER       | 0         |
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
| debian13-vm | RUNNING | 10.104.236.239 (enp5s0) | fd42:7786:2217:a97c:1266:6aff:fe57:2656 (enp5s0) | VIRTUAL-MACHINE | 0         |
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
```

Ensure all instances are stopped before powering down the system to install IncusOS:

```
for instance in $(incus list --columns n --format compact,noheader); do incus stop $instance; done
```

## Install IncusOS

Follow the [instructions to install IncusOS](../getting-started/installation.md) on the server.

When IncusOS boots, you will see a warning about it refusing to import the existing ZFS pool because it is unencrypted:

```
2025-11-12 16:27:39 INFO Bringing up the local storage
2025-11-12 16:27:39 WARN Refusing to import unencrypted ZFS pool 'oldpool'
```

## Create a new encrypted ZFS storage pool

Once IncusOS is installed, we will create a new ZFS pool `newpool` via the IncusOS API. In this tutorial, for simplicity it will consist of a single drive, but more complex/robust pool configuration is possible.

`oldpool` exists on `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1` and `newpool` will be created on `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2`.

Show the system's current storage state:

```
gibmat@futurfusion:~$ incus admin os system show storage
WARNING: The IncusOS API and configuration is subject to change

config: {}
state:
  drives:
  - boot: false
    bus: scsi
    capacity_in_bytes: 1.610612736e+10
    id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
    model_family: QEMU
    model_name: QEMU HARDDISK
    remote: false
    removable: false
    serial_number: incus_disk1
  - boot: false
    bus: scsi
    capacity_in_bytes: 1.610612736e+10
    id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
    model_family: QEMU
    model_name: QEMU HARDDISK
    remote: false
    removable: false
    serial_number: incus_disk2
  - boot: true
    bus: scsi
    capacity_in_bytes: 5.36870912e+10
    id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root
    model_family: QEMU
    model_name: QEMU HARDDISK
    remote: false
    removable: false
    serial_number: incus_root
  pools:
  - devices:
    - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
    encryption_key_status: available
    name: local
    pool_allocated_space_in_bytes: 4.276224e+06
    raw_pool_size_in_bytes: 1.7716740096e+10
    state: ONLINE
    type: zfs-raid0
    usable_pool_size_in_bytes: 1.7716740096e+10
```

Create `newpool`:

`incus admin os system edit storage`

```
config: 
  pools:    
  - name: newpool
    devices:
    - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
    type: zfs-raid0
```

Show that the new ZFS pool is indeed created:

```
gibmat@futurfusion:~/incus$ ./incus admin os system show storage
WARNING: The IncusOS API and configuration is subject to change

config: {}
state:
  drives:
  - boot: false
    bus: scsi
    capacity_in_bytes: 1.610612736e+10
    id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1
    model_family: QEMU
    model_name: QEMU HARDDISK
    remote: false
    removable: false
    serial_number: incus_disk1
  - boot: false
    bus: scsi
    capacity_in_bytes: 1.610612736e+10
    id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
    member_pool: newpool
    model_family: QEMU
    model_name: QEMU HARDDISK
    remote: false
    removable: false
    serial_number: incus_disk2
  - boot: true
    bus: scsi
    capacity_in_bytes: 5.36870912e+10
    id: /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root
    model_family: QEMU
    model_name: QEMU HARDDISK
    remote: false
    removable: false
    serial_number: incus_root
  pools:
  - devices:
    - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_root-part11
    encryption_key_status: available
    name: local
    pool_allocated_space_in_bytes: 4.276224e+06
    raw_pool_size_in_bytes: 1.7716740096e+10
    state: ONLINE
    type: zfs-raid0
    usable_pool_size_in_bytes: 1.7716740096e+10
  - devices:
    - /dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk2
    encryption_key_status: available
    name: newpool
    pool_allocated_space_in_bytes: 598016
    raw_pool_size_in_bytes: 1.5569256448e+10
    state: ONLINE
    type: zfs-raid0
    usable_pool_size_in_bytes: 1.5569256448e+10

```

## Create a new Incus storage pool

Next, create a new Incus storage pool called `incus_new` using the new ZFS pool:

```
gibmat@futurfusion:~$ incus storage create incus_new zfs source=newpool
Storage pool incus_new created
gibmat@futurfusion:~$ incus storage list
+-----------+--------+--------------------------------------+---------+---------+
|   NAME    | DRIVER |             DESCRIPTION              | USED BY |  STATE  |
+-----------+--------+--------------------------------------+---------+---------+
| incus_new | zfs    |                                      | 0       | CREATED |
+-----------+--------+--------------------------------------+---------+---------+
| local     | zfs    | Local storage pool (on system drive) | 3       | CREATED |
+-----------+--------+--------------------------------------+---------+---------+

```

## Use `incus admin recover` to import existing instances

```{warning}
FIXME: Need to update the Incus API to be able to run this remotely.
```

```
bash-5.2# incus admin recover
This server currently has the following storage pools:
 - incus_new (backend="zfs", source="newpool")
 - local (backend="zfs", source="local/incus")
Would you like to recover another storage pool? (yes/no) [default=no]: yes
Name of the storage pool: incus
Name of the storage backend (dir, zfs): zfs
Source of the storage pool (block device, volume group, dataset, path, ... as applicable): oldpool
Additional storage pool configuration property (KEY=VALUE, empty when done): 
Would you like to recover another storage pool? (yes/no) [default=no]: 
The recovery process will be scanning the following storage pools:
 - EXISTING: "incus_new" (backend="zfs", source="newpool")
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

## Move existing instances to new storage pool

Now that we have both the old and new ZFS pools avaiable, we can move the instances from the unencrypted `oldpool` to the encrypted `newpool`:

`for instance in $(incus list --columns n --format compact,noheader); do incus move $instance $instance --storage incus_new; done`

Once complete, delete the old ZFS pool:

```
gibmat@futurfusion:~$ incus storage delete incus
Storage pool incus deleted
gibmat@futurfusion:~$ incus storage list
+-----------+--------+--------------------------------------+---------+---------+
|   NAME    | DRIVER |             DESCRIPTION              | USED BY |  STATE  |
+-----------+--------+--------------------------------------+---------+---------+
| incus_new | zfs    |                                      | 2       | CREATED |
+-----------+--------+--------------------------------------+---------+---------+
| local     | zfs    | Local storage pool (on system drive) | 3       | CREATED |
+-----------+--------+--------------------------------------+---------+---------+
```

## Start instances and verify running

Now you can start the migrated instances on the IcusOS server with the encrypted storage:

```
gibmat@futurfusion:~$ for instance in $(incus list --columns n --format compact,noheader); do incus start $instance; done
gibmat@futurfusion:~$ incus list
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
|    NAME     |  STATE  |          IPV4           |                       IPV6                       |      TYPE       | SNAPSHOTS |
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
| debian13    | RUNNING | 10.180.174.121 (eth0)   | fd42:1b66:c564:e8f2:1266:6aff:fe30:bb19 (eth0)   | CONTAINER       | 0         |
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+
| debian13-vm | RUNNING | 10.180.174.239 (enp5s0) | fd42:1b66:c564:e8f2:1266:6aff:fe57:2656 (enp5s0) | VIRTUAL-MACHINE | 0         |
+-------------+---------+-------------------------+--------------------------------------------------+-----------------+-----------+

```

## Wipe old disk(s)

Finally, you can wipe the disk(s) that composed the old, unencrypted storage pool:

```
gibmat@futurfusion:~/$ incus admin os system wipe-drive -d '{"id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_disk1"}'
WARNING: The IncusOS API and configuration is subject to change

Are you sure you want to wipe the drive? (yes/no) [default=no]: yes
```

Once complete they can be used to create another pool or extend an existing one. Or you can physically remove them from the IncusOS server.
