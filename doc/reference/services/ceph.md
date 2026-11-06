# Ceph

The [Ceph](https://ceph.io/) service allows connecting a Ceph storage cluster. In addition to Incus, the `incus-ceph` application must be installed to enable this service.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_ceph.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Ceph service.

* `clusters`: A map of Ceph clusters to connect to, with the cluster name as the key. Each cluster consists of:

   * `fsid`: The unique identifier of the Ceph cluster.

   * `monitors`: An array of Ceph monitor addresses.

   * `keyrings`: A map of Ceph client keyrings, with the client name as the key. Each keyring consists of:

      * `key`: The secret key of the client.

   * `client_config`: An optional map of Ceph configuration options to apply to all clients of the cluster.

### Examples

Connect to a cluster named `ceph` using the `admin` client:

```yaml
config:
  enabled: true
  clusters:
    ceph:
      fsid: "0e0a8ae5-6a5d-4d7a-9c0b-6b3f1c2d4e5f"
      monitors:
      - "10.0.0.11"
      - "10.0.0.12"
      - "10.0.0.13"
      keyrings:
        admin:
          key: "AQBvaBlnAAAAABAAl2x0hJ1sY8Z6m3bX0n8vVQ=="
      client_config:
        rbd_cache: "true"
```
