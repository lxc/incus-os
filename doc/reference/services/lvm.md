# LVM

The LVM service allows configuring of a clustered LVM storage backend.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_lvm.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the LVM service.

* `system_id`: A cluster-unique host identifier.

## Lock loss and fencing

Clustered LVM protects shared volumes with `sanlock` leases stored on the shared storage itself.
Every server renews its lease continuously and any server that stops renewing is considered dead by the others once its lease expires.

```{warning}
A server that loses access to the shared storage, for example through a storage network outage or a storage array stall, can't renew its lease.
After 80 seconds of failed renewals, `sanlock` revokes the server's locks, which makes every active logical volume on that server unprotected.
If the storage doesn't come back within about a minute after that, the `wdmd` watchdog forcefully resets the server to prevent any further writes to the shared volumes.
```

When the storage comes back before the watchdog fires, IncusOS automatically recovers by dropping and restarting the lockspace, then re-acquiring the locks of all active logical volumes with the mode they previously held.
Instances keep running during the outage and the recovery, but their I/O stalls for the duration of the outage.

Should a lock no longer be available at that point, meaning another server now owns the volume, IncusOS immediately resets the server, like the watchdog would, rather than risk corrupting the shared volume.
