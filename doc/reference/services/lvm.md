# LVM

The LVM service allows configuring of a clustered LVM storage backend.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_lvm.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the LVM service.

* `system_id`: A cluster-unique host identifier.

* `io_timeout`: The `sanlock` I/O timeout in seconds, defaults to `10`.
  Each server renews its lease every two times this value and loses its locks after eight times this value of failed renewals.

* `watchdog_timeout`: The watchdog timeout in seconds, defaults to `60`.
  The hardware watchdog must support the value and all servers of the cluster must use the same one.
  `sanlock` recommends lowering it together with `io_timeout`, for example `30` with an `io_timeout` of `5`.

Changing the timeouts or the host identifier restarts the service, which fails while any shared logical volume is active.

## Debugging

When the service is enabled, the full physical volume, volume group and logical volume reports can be retrieved through the `/1.0/debug/lvm` API endpoint or with `incus admin os debug lvm`.
The command shows a summary of each report, use `--format json` or `--format yaml` to get the full reports.

The reports are read from the on-disk LVM metadata only, without taking any lock, so they remain available while the lock manager is unavailable.
As a result, the device status fields (such as whether a logical volume is active) aren't populated and the attribute strings show `X` in their place.
All sizes are reported in bytes.

## Lock loss and fencing

Clustered LVM protects shared volumes with `sanlock` leases stored on the shared storage itself.
Every server renews its lease continuously and any server that stops renewing is considered dead by the others once its lease expires.

```{warning}
A server that loses access to the shared storage, for example through a storage network outage or a storage array stall, can't renew its lease.
After 80 seconds of failed renewals with the default timeouts, `sanlock` revokes the server's locks, which makes every active logical volume on that server unprotected.
If the storage doesn't come back within the watchdog timeout after that, the `wdmd` watchdog forcefully resets the server to prevent any further writes to the shared volumes.
```

When the storage comes back before the watchdog fires, IncusOS automatically recovers by dropping and restarting the lockspace, then re-acquiring the locks of all active logical volumes with the mode they previously held.
Instances keep running during the outage and the recovery, but their I/O stalls for the duration of the outage.

Should a lock no longer be available at that point, meaning another server now owns the volume, IncusOS immediately resets the server, like the watchdog would, rather than risk corrupting the shared volume.

## Global storage outages

During a storage outage that affects every server, such as the storage array rebooting or the storage network going down, the watchdog resets all servers even though none of them can write to the shared volumes.
To prevent this, a global storage outage can be declared on each server through the `declare-outage` action, either ahead of planned work or while the outage is ongoing.

```
incus admin os service action lvm -d '{"action":"declare-outage"}'
```

An optional `timeout` parameter in seconds, defaulting to `3600`, sets how long the declaration stays armed.

```
incus admin os service action lvm -d '{"action":"declare-outage","config":{"timeout":"7200"}}'
```

A declaration has no effect on a server that keeps access to the storage.
On a server that loses its lease while the declaration is armed, IncusOS suspends all active logical volumes of the volume group and drops its lockspace, which disarms the watchdog.
Instances keep running but their I/O stalls until the storage comes back.

Once the storage is back, IncusOS restarts the lockspace, re-acquires the locks of the logical volumes with the mode they previously held, resumes them and clears the declaration.
Should a lock no longer be available at that point, IncusOS resets the server as described above.

A declaration that didn't take effect expires after its timeout, doesn't persist across reboots and can be withdrawn at any time through the `clear-outage` action.

```
incus admin os service action lvm -d '{"action":"clear-outage"}'
```

The current declaration and the affected volume groups can be seen in the `outage` section of the service state.
The other state sections aren't refreshed while a declaration is in place as that would block on the storage.

```{warning}
A server gets reset 110 to 130 seconds after the storage goes down with the default timeouts, so a declaration made during an outage must reach every server before that.
```

```{warning}
Only declare an outage if no server can reach the shared storage.
A server that merely got partitioned from the storage isn't fenced when the declaration is armed, so another server could start using its volumes while its own writes are still in flight.
Those writes may reach the storage when it comes back, before IncusOS detects the conflict, which can corrupt the volume.
```

NVMe over Fabrics controllers don't time out while the outage is in effect, so they reconnect no matter how long it lasts.
Other storage must be configured to queue I/O for at least as long as an outage may last, otherwise the I/O of the instances fails.
