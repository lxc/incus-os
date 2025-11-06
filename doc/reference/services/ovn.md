# {abbr}`OVN (Open Virtual Network)`

The [{abbr}`OVN (Open Virtual Network)`](https://www.ovn.org/) service allows configuring an OVN software defined network.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_ovn.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the OVN service.

* `ic_chassis`: Boolean indicating if the chassis is used as an interconnection gateway.

* `database`: The OVN database that the system should connect to for its configuration.

* `tls_client_certificate`: A PEM-encoded client certificate.

* `tls_client_key`: A PEM-encoded client key.

* `tls_ca_certificate`: A PEM-encoded CA certificate.

* `tunnel_address`: The IP address that a chassis should use to connect to this node using encapsulation types specified by `tunnel_protocol`. Multiple encapsulation IPs may be specified with a comma-separated list.

* `tunnel_protocol`: The encapsulation type that a chassis should use to connect to this node. Multiple encapsulation types may be specified with a comma-separated list.
