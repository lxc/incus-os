# Shared API

Each IncusOS application shares a common API that can be used to restart it if needed as well as perform backup, restore, and reset operations.

## Restarting the application

If needed, an application can be restarted by running

```
incus admin os application restart <name>
```

```{note}
It is expected to receive an EOF error since the application's HTTP REST endpoint will be restarted along with the application.
```

## Backing up the application

```{important}
An IncusOS application backup may contain sensitive data and credentials. As such, the backup should not be stored in any publicly-accessible location.
```

A backup of the application can be created which will include its state and configuration. Optionally, a complete backup can be created which will include all locally cached artifacts or updates.

### Configuration options

* `complete`: If `true`, a full backup will be generated which may be quite large depending on what artifacts or updates are locally cached by the application.

### Examples

Create the backup by running

```
incus admin os application backup <name> archive.tar.gz -d '{"complete":false}'
```

## Restoring the application

```{warning}
Restoring a backup will overwrite any existing application state. As such, use caution when restoring.
```

Restore the backup by running

```
incus admin os application restore <name> backup.tar.gz
```

```{note}
It is expected to receive an EOF error since the application's HTTP REST endpoint will be restarted along with the application after performing the restoration.
```

## Factory reset

```{warning}
A factory reset will erase all configuration and state for the application.
```

Reset the application by running

```
incus admin os application factory-reset <name>
```

```{note}
It is expected to receive an EOF error since the application's HTTP REST endpoint will be restarted along with the application after resetting the application.
```
