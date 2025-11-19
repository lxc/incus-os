# Linstor

The [Linstor](https://linbit.com/linstor/) service allows connecting a Linstor deployment. In addition to Incus, the `incus-linstor` application must be installed to enable this service.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_linstor.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Linstor service.

* `listen_address`: The address and port to listen on (default to all addresses).

* `tls_server_certificate`: When using TLS, the server certificate to use.

* `tls_server_key`: When using TLS, the server key to use.

* `tls_trusted_certificates`: When using TLS, the list of certificates to trust.
