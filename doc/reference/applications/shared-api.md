# Shared API

Each IncusOS application shares a common API that can be used to stop, start or restart it if needed as well as perform backup, restore, and reset operations.

## Listing the applications

```
incus admin os application list
```

## Getting the application state

```
incus admin os application show <name>
```

## Editing the application configuration

```
incus admin os application edit <name>
```

## Checking for updates

Application updates are normally applied as part of the regular IncusOS update checks. A check for an update of a single application can also be manually triggered, with the update being applied if one is found, by running

```
incus admin os application check-update <name>
```

## Restarting the application

If needed, an application can be restarted by running

```
incus admin os application restart <name>
```

```{note}
It is expected to receive an EOF error since the application's HTTP REST endpoint will be restarted along with the application.
```

## Stopping and starting the application

An application can be stopped and later started again by running

```
incus admin os application stop <name>
incus admin os application start <name>
```

A stopped application stays stopped until it is started again or the system is rebooted.

The IncusOS API is normally reached through the primary application, so the primary application can only be stopped while the [fallback listener](../system/fallback-listener.md) is active. The fallback listener must then be used to start the application again.

## Changing application version

On rare occasions, it might be necessary to temporarily rollback an application to an earlier version. This can be accomplished via the `switch-version` command

```
incus admin os application switch-version <name> [<version>]
```

where `<version>` is one of the available application versions as reported by the `application show` command.

If no version is specified, IncusOS will attempt to rollback the application to the prior available version.

Changing application versions are limited to those that are available locally on disk.

If IncusOS is booted into the backup image, it will automatically attempt to re-synchronize application versions to match the older version of IncusOS. Upon rebooting to the non-backup image, application versions will be updated to the latest available versions locally on disk.

## Pinning current versions

It is possible to "pin" the available versions of an application by setting the `pin_current_versions` field to true in the application's configuration. While the pin is enabled, no newer versions of the application will be downloaded.

```{warning}
When an application's versions are pinned, it will not receive further bug/security fixes or newer releases.
```

## Performing application actions

Some applications support additional actions, which are listed on the page of the application. An action is triggered by providing its name and an optional map of parameters

```
incus admin os application action <name> -d '{"action":"<action>","config":{"<key>":"<value>"}}'
```

Applications may also provide debug actions, which are run the same way through the `debug` command

```
incus admin os application debug <name> -d '{"action":"<action>"}'
```

## Installing a specific application version

In rare cases, it may be necessary to install a specific version of an application. This can be achieved by setting the `force_version` field to the required IncusOS version string desired. (It's not possible to directly specify an application-specific version via this command.)

```
incus admin os application add <application> -d '{"force_version":"202610281429"}'
```

## Forcefully reinstalling an application

In rare cases, it may be necessary to forcefully reinstall an existing application. This can be achieved by setting the `force_reinstall` field to true and re-running the `application add` command. Optionally, you can also specify a `force_version` value to reinstall a specific version, rather than defaulting to the latest available version.

```
incus admin os application add <application> -d '{"force_reinstall":true}'
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

## Removing the application

```{warning}
Removing an application will erase all configuration and state for the application.
```

A non-primary application that is no longer needed can be removed by running

```
incus admin os application remove <name>
```
