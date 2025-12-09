# Logging

IncusOS can be configured to log to a remote syslog server.

## Configuration options

Configuration fields are defined in the [`SystemLoggingSyslog` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_logging.go).

The following configuration options can be set:

* `address`: The remote syslog server IP address.

* `protocol`: The protocol to use when connecting to the remote syslog server.

* `log_format`: The format of log entries to use.
