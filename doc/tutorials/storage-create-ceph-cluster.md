# Creating a Ceph cluster

IncusOS can automate the process of deploying a Ceph cluster. Utilizing OCI container images, IncusOS can self-host a resilient and fault-tolerant Ceph storage cluster.

## Requirements

* An [IncusOS cluster](./incus-cluster.md) consisting of at least three servers:

   * Each server should have a storage pool named `local` available to store Ceph OCI image(s)

   * An Incus-managed common network accessible to all cluster servers with IPv6 connectivity; ideally this network should only be available to cluster servers

   * Optionally, a dedicated project to use when creating the Ceph OCI images and containers

* Each cluster server that will host a Ceph OSD must have an unused raw block device available for the OSD to use. IncusOS will setup LUKS encryption of the raw device before passing it through to the OSD container.

An IncusOS cluster deployed by Operations Center should meet all of these requirements out of the box.

## Initializing the Ceph control plane

The first step is to create the Ceph control plane. The Ceph OCI image will be downloaded and three Ceph containers will be created, each on a different Incus cluster server. The initialization logic also ensures the `incus-ceph` application is installed on each member of the Incus cluster, and that the Ceph service is properly configured.

```
incus admin os application action incus -d '{"action":"initialize-ceph-cluster","config":{"control_servers":"server01,server02,server03"}}'
```

### Configuration parameters

* `control_servers`: A comma-separated list of three Incus servers to use when initially deploying the Ceph control plane. If not specified, defaults to the first three Incus cluster servers as reported via the Incus API.
* `network`: The Incus network that should be utilized by the Ceph cluster. If not specified, defaults to `meshbr0`.

## Adding storage drives (OSDs)

Ceph requires a minimum of three OSDs, and only one OSD can run at a time on a given physical Incus server to help ensure proper replication. Because of this requirement, the OSD containers won't actually be created until the third OSD is defined, at which point all three OSDs are initialized. Addition of subsequent OSDs is performed immediately.

Each OSD is bound to a specific Incus server, so the `--target` parameter must be specified when creating a new OSD.

```
incus admin os application action incus-ceph -d '{"action":"add-drive","config":{"device_id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk"}}' --target server01
incus admin os application action incus-ceph -d '{"action":"add-drive","config":{"device_id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk"}}' --target server02
incus admin os application action incus-ceph -d '{"action":"add-drive","config":{"device_id":"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk"}}' --target server03
```

### Configuration parameters

* `device_id`: The device ID of the raw block device to be used by the OSD.

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

## Removing a storage drive (OSD)

If four or more OSDs are available, an OSD can be removed from the Ceph configuration. Because each OSD is bound to a specific Incus server, the `--target` parameter must be specified:

```{warning}
Removing an OSD via the IncusOS API is equivalent to pulling the power cord of a physical server. Prior to removal of the OSD container, be sure to properly remove the OSD from Ceph's configuration and allow sufficient time for any necessary data migrations to complete.
```

```
incus admin os application action incus-ceph -d '{"action":"remove-drive"}' --target server01
```

## Ceph cluster configuration and status

After the Ceph cluster is initialized, the IncusOS `ceph` service will report basic status information:

```
$ incus admin os service show ceph
WARNING: The IncusOS API and configuration is subject to change

config:
  clusters:
    ceph:
      client_config: null
      fsid: 7fb7b08a-ec49-4ec0-abb0-cd4e9b7089bb
      keyrings:
        admin:
          key: AQD+6U9qzwiAEhAAriOdhjIAN53Oson6D0c1yg==
      monitors:
        - fd42:98b5:52c7:673d:1266:6aff:fefa:2cca
        - fd42:98b5:52c7:673d:1266:6aff:fe4b:2007
        - fd42:98b5:52c7:673d:1266:6aff:fe0a:310f
  enabled: true
state:
  election_epoch: 22
  fsid: 7fb7b08a-ec49-4ec0-abb0-cd4e9b7089bb
  fsmap:
    btime: 2026-07-09T19:27:42:293081+0000
    epoch: 5
  health:
    status: HEALTH_OK

[snip]


```

A minimal amount of configuration state is stored in Incus cluster-wide project configuration using keys with the `user.ceph.*` prefix. This is used to manage the deployment of OSDs at the Incus cluster level:

```
$ incus project show internal
config:
  user.ceph.fsid: 7fb7b08a-ec49-4ec0-abb0-cd4e9b7089bb
  user.ceph.network: meshbr0
  user.ceph.osds: '[{"host":"server01","device_id":"/dev/mapper/luks-scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk","device_class":"sdd"},{"host":"server02","device_id":"/dev/mapper/luks-scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk","device_class":"sdd"},{"host":"server03","device_id":"/dev/mapper/luks-scsi-0QEMU_QEMU_HARDDISK_incus_ceph-disk","device_class":"sdd"}]'

```
