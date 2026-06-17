# Creating a Ceph cluster

IncusOS can automate the process of deploying a Ceph cluster. Utilizing OCI container images, IncusOS can self-host a resilient and fault-tolerant Ceph storage cluster.

## Requirements

* An [IncusOS cluster](./incus-cluster.md) consisting of at least three servers:

   * Each server should have a storage pool named `local` available to store Ceph OCI image(s)

   * An Incus-managed common network accessible to all cluster servers with IPv6 connectivity; ideally this network should only be available to cluster servers

   * Optionally, a dedicated project to use when creating the Ceph OCI images and containers

* Each cluster server that will host a Ceph OSD must have an unused raw block device available for the OSD to use. IncusOS will setup LUKS encryption of the raw device before passing it through to the OSD container.

An IncusOS cluster deployed by Operations Center should meet all these requirements out of the box.

## Initializing the Ceph control plane

The first step is to create the Ceph control plane. The Ceph OCI image will be downloaded and three Ceph containers will be created, each on a different Incus server. Additionally, the `incus-ceph` application will be installed on each cluster server if not already present.

```
incus admin os application action incus -d '{"action":"initialize-ceph-cluster","config":{"control_servers":"server01,server02,server03"}}'
```

### Configuration parameters

* `control_servers`: A comma-separated list of three Incus servers to use when initially deploying the Ceph control plane. If not specified, defaults to the first three Incus cluster servers as reported via the Incus API.
* `network`: The Incus network that should be utilized by the Ceph cluster. If not specified, defaults to `meshbr0`.
* `project`: The Incus project that should be utilized by the Ceph cluster. If not specified, defaults to `internal`.

## Adding OSDs

A minimum of three OSDs are required, and only one OSD can run at a time on a given Incus server. Because of this requirement, the OSD containers won't actually be created until the third OSD is defined, at which point all three OSDs are initialized. Addition of subsequent OSDs is performed immediately.

Each OSD is bound to a specific Incus server, so the `--target` parameter must be specified when creating a new OSD.

```
incus admin os application action incus-ceph -d '{"action":"add-osd","config":{"device_id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk"}}' --target server01
incus admin os application action incus-ceph -d '{"action":"add-osd","config":{"device_id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk"}}' --target server02
incus admin os application action incus-ceph -d '{"action":"add-osd","config":{"device_id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk"}}' --target server03
```

### Configuration parameters

* `device_id`: The device ID of the raw block device to be used by the OSD.

## Adding a new Incus storage pool

As a convenience, IncusOS can automatically create a new Incus Ceph storage pool:

```
incus admin os application action incus-ceph -d '{"action":"create-pool"}'
```

### Configuration parameters

* `pool_name`: The name of the new Incus pool. If not specified, defaults to `ceph`.

## Updating deployed Ceph version

Because Ceph is deployed using OCI images, it's easy to update or change the running version of Ceph. By default, IncusOS creates the Ceph containers using a major version tag, such as `v20`. As the upstream tag is changed to track the latest release, refreshing and rebuilding the Ceph containers will then automatically pick up that version. It is also possible to specify an explicit tag such as `v21` to bump Ceph to the next major version, `v20.2` to more specifically track a series of Ceph releases, or `v20.1.1` to pin to a specific version.

```{note}
IncusOS will not automatically remove old, unused Ceph OCI images; this must be done manually.
```

### Configuration parameters

* `oci_tag`: If specified, update the Ceph containers to use the specific OCI image tag; otherwise, refresh the currently used OCI image and update the Ceph containers if a newer version is available.

```
incus admin os application action incus-ceph -d '{"action":"refresh-images"}'
```

## Removing an OSD

If four or more OSDs are available, an OSD can be removed from the Ceph configuration. Because each OSD is bound to a specific Incus server, the `--target` parameter must be specified:

```{warning}
Removing an OSD via the IncusOS API is equivalent to pulling the power cord of a physical server. Prior to removal of the OSD container, be sure to properly remove the OSD from Ceph's configuration and allow sufficient time for any necessary data migrations to complete.
```

```
incus admin os application action incus-ceph -d '{"action":"remove-osd"}' --target server01
```

## Ceph cluster configuration and status

After the Ceph cluster is created and an Incus storage pool defined, the IncusOS `ceph` service will report basic status information:

```
$ incus admin os service show ceph
WARNING: The IncusOS API and configuration is subject to change

config:
  clusters:
    ceph:
      client_config: null
      fsid: 90dbdb08-1734-462a-99a8-3f16d986fa3d
      keyrings:
        admin:
          key: AQDc8DJqE/UgJRAAFCxini1juAbFHqteIYtXsA==
      monitors:
        - fd42:3cff:25bf:5b9d:1266:6aff:fef7:1ae2
        - fd42:3cff:25bf:5b9d:1266:6aff:fe94:92e5
        - fd42:3cff:25bf:5b9d:1266:6aff:fefc:44eb
  enabled: true
state:
  cluster:
    health: HEALTH_OK
    id: 90dbdb08-1734-462a-99a8-3f16d986fa3d
  data:
    objects: 7 objects, 449 KiB
    pgs: 33 active+clean
    pools: 2 pools, 33 pgs
    usage: 81 MiB used, 150 GiB / 150 GiB avail
  services:
    mgr: 'ceph-central01(active, since 8m), standbys: ceph-central02, ceph-central03'
    mon: '3 daemons, quorum ceph-central01,ceph-central02,ceph-central03 (age 7m)
      [leader: ceph-central01]'
    osd: '3 osds: 3 up (since 2m), 3 in (since 2m)'
    rbd-mirror: 3 daemons active (3 hosts)


```

A minimal amount of configuration state is stored in the Incus cluster-wide configuration using keys with the `user.ceph.*` prefix. This is used to manage the deployment of OSDs at the Incus cluster level:

```
$ incus config show
config:
  user.ceph.project: internal

$ incus project show internal
config:
  user.ceph.fsid: 90dbdb08-1734-462a-99a8-3f16d986fa3d
  user.ceph.network: meshbr0
  user.ceph.osds: '[{"host":"server01","device_id":"/dev/mapper/luks-scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk","device_class":"sdd"},{"host":"server02","device_id":"/dev/mapper/luks-scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk","device_class":"sdd"},{"host":"server03","device_id":"/dev/mapper/luks-scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk","device_class":"sdd"}]'

```
