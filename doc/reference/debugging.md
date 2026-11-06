# Debugging

IncusOS doesn't provide any shell access, so the state of a system is inspected through its API instead. The commands listed here cover the most common needs when investigating an issue.

```{warning}
The IncusOS debug API endpoints have no guarantee of API stability, and should not be used in normal day-to-day operations.
```

## System overview

Basic details about the system, such as its name, running and next version, how long it has been running and whether it is done starting up, can be retrieved by running

```
incus admin os show
```

A more complete summary, which also includes the hardware, network addresses, installed applications, the recent IncusOS log entries and any warnings, can be retrieved by running

```
incus admin os info
```

## System logs

The system journal can be retrieved by running

```
incus admin os debug log
```

The following options can be used to filter the returned entries:

* `--unit`: Limit the entries to the specified systemd unit.

* `--boot`: Limit the entries to the specified boot number, with `0` being the current boot and `-1` the previous one.

* `--entries`: Limit the number of returned entries.

* `--since` and `--until`: Limit the entries to those logged after or before the specified date and time.

For example, show the log entries of the IncusOS daemon since the system started by running

```
incus admin os debug log --unit incus-osd --boot 0
```

## Processes

The list of all the processes running on the system can be retrieved by running

```
incus admin os debug processes
```

## Kernel

The kernel version, architecture, processor details and the list of loaded kernel modules can be retrieved by running

```
incus admin os debug kernel
```

## System metrics

A report of the system metrics can be retrieved by running

```
incus admin os debug report
```

The report can be limited to specific metric families or prefixes by passing the `--match` option, which may be repeated.

## LVM

The LVM physical volume, volume group and logical volume reports can be retrieved by running

```
incus admin os debug lvm
```

See the [LVM service](services/lvm.md) for details.

## Secure Boot

The TPM event log for the PCRs that IncusOS relies on, along with their final values, can be retrieved by running

```
incus admin os debug secureboot event-log
```

IncusOS automatically applies [Secure Boot key updates](security.md). For testing purposes, a tar archive of Secure Boot variable updates can also be manually applied by running

```
incus admin os debug secureboot update <file>
```

The system reboots on its own if the update requires it.

## Applications

Applications can provide their own debug actions, such as retrieving the logs of [Incus](applications/incus.md). See the [shared application API](applications/shared-api.md) for how to run them.

## Signed debug scripts

As a last resort, the IncusOS developers can provide a shell script to collect additional information or recover a system. Such a script is S/MIME-signed and IncusOS only runs it after verifying its signature.

Run the signed script and get its output by running

```
incus admin os debug run-script <file>
```
