# Update

IncusOS will check the configured [provider](providers.md) for stable updates at each boot and then by default every six hours thereafter. When an update is available, IncusOS will download it, update/restart any applications, and stage the OS update for next boot. Occasionally a Secure Boot key update may also be published, which will be automatically applied before any other available updates.

Updates to the stable channel are normally published once a week to pick up the latest stable bug fix release of the Linux kernel as well as any relevant security issues, while the testing channel may see more frequent updates as new features are developed. It is generally recommended to remain on the stable channel.

When an OS update is installed, IncusOS will display a message on the console that a reboot is required to finish applying the update. It will also report this via the REST API update state when queried.

## Configuration options

The following configuration options can be set:

* `auto_reboot`: If `true`, IncusOS will automatically restart itself after applying an update. Note that this will cause some period of service interruption for any applications running on that server while it reboots. (IncusOS will always automatically reboot if it applies an update on system boot.)

* `channel`: Either `stable` or `testing`.

* `check_frequency`: A string that is parsable as a duration by Go's `time.ParseDuration()` or the special value `never`. Controls the frequency that IncusOS will use when checking for updates. Setting to `never` disables any automatic updates; this is typically discouraged as the system will be dependent on manual update checks to receive any security updates.

* `maintenance_windows`: An optional list of maintenance windows.

## Maintenance windows

IncusOS supports defining maintenance windows that limit when the system will check for and apply updates. This can be useful to prevent updates from being installed during normal business hours or other inconvenient times. Each maintenance window consists of a start time and an end time (assumed to be in the system's configured timezone) and an optional start day of week and end day of week.

### Examples

Allow updates daily each night between 10pm - 6am:

```
{
    "start_hour": 22,
    "start_minute": 0,
    "end_hour": 6,
    "end_minute": 0
}
```

Allow updates only on the weekend:

```
{
    "start_day_of_week": "Saturday",
    "start_hour": 0,
    "start_minute": 0,
    "end_day_of_week": "Sunday",
    "end_hour": 23,
    "end_minute": 59
}
```

## Manually checking for an update

You can instruct IncusOS to check for an update at any time by running

```
incus admin os system update check
```
