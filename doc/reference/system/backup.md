# Backup/Restore

IncusOS can perform a system-level backup of its configuration and state. This backup can then be restored at a later point in time. Additionally, a full factory reset can be performed which will bring IncusOS back to a clean state as if it had just been installed.

```{important}
The system-level backup doesn't include information from any installed applications. Each application has its own backup/restore functionality which can be used in conjunction with the system-level backup to perform a comprehensive system backup.
```

## Backup

```{important}
An IncusOS backup will contain its current state as well as copies of the encryption key(s) for any local storage pool(s). As such, the backup should not be stored in any publicly-accessible location.
```

Create the backup by running

```
incus admin os system backup backup.tar.gz
```

## Restore

```{warning}
Restoring a backup will overwrite any existing OS-level state and potentially one or more encryption keys. As such, use caution when restoring.
```

### Configuration options

The following "skip" options can be set when restoring a backup:

* `encryption-recovery-keys`: Don't overwrite any existing main system drive encryption recovery keys.

* `local-data-encryption-key`: Don't overwrite the local storage pool's encryption key.

* `network-macs`: Don't use any hard-coded MACs from the backup, but rather attempt to determine the proper MACs from the existing interfaces.

### Examples

Restore the backup by running

```
incus admin os system restore backup.tar.gz
```

## Factory reset

```{warning}
A factory reset will erase all data on the main system drive. This includes any installed applications, their configuration and the system-level state and configuration.

User-created local storage pools will be untouched, but will be unable to be imported when the system reboots. Be certain you have a copy of each local storage pool's encryption key __before__ performing the factory reset.
```

### Configuration options

The following configuration options can be set when performing a factory reset:

*`allow_tpm_reset_failure`: If `true`, ignore failures when resetting TPM state.

* `seeds`: A map of seeds to write to the seed partition just before rebooting the system. This can be useful to change/update existing seed data when the system configures itself after booting.

* `wipe_existing_seeds`: If `true`, wipe any existing seed data that may be present in the seed partition.

### Examples

Perform a basic reset that will reuse any existing seed data by running

```
incus admin os system factory-reset
```

Perform a reset that allows TPM failure, wipes any existing seeds, and configures a basic Incus application upon reboot by running

```
incus admin os system factory-reset -d '{"allow_tpm_reset_failure":true,"wipe_existing_seeds":true,"seeds":{"incus":{"apply_defaults":true}}}'
```
