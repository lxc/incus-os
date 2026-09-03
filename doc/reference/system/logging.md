# Logging

IncusOS can be configured to log to a remote syslog server and/or to upload
its journal to a remote server compatible with `systemd-journal-remote`.

## Configuration options

Configuration fields are defined in the [`SystemLoggingConfig` struct](https://github.com/lxc/incus-os/blob/main/incus-osd/api/system_logging.go).

### Syslog

The following configuration options can be set under `syslog`:

* `address`: The remote syslog server IP address.

* `protocol`: The protocol to use when connecting to the remote syslog server.

* `log_format`: The format of log entries to use.

### Journal upload

The following configuration options can be set under `journal_upload`:

* `url`: The URL of the remote server to upload the journal to.

* `tls_client_certificate`: An optional PEM-encoded client certificate used to authenticate to the remote server.

* `tls_client_key`: The PEM-encoded private key matching `tls_client_certificate`.

* `tls_ca_certificate`: An optional PEM-encoded CA certificate used to verify the remote server.
  When unset, the system CA bundle is used.
