# NetBird

The [NetBird](https://netbird.io/) service allows configuring a NetBird VPN client.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_netbird.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the NetBird service.

* `setup_key`: A NetBird setup-key.

* `management_url`: The NetBird management server.

* `admin_url`: The NetBird admin server.

* `anonymize`: If `true`, anonymize IP addresses and non-netbird.io domains in logs and status output.

* `block_inbound`: If `true`, do not allow any inbound connections.

* `block_lan_access`: If `true`, block access to local networks when using this peer as a router or exit node.

* `disable_client_routes`: If `true`, the client won't process routes received from the management service.

* `disable_server_routes`: If `true`, the client won't act as a router for server routes received from the management service.

* `disable_dns`: If `true`, the client won't configure DNS settings.

* `disable-firewall`: If `true`, the client won't modify firewall rules.

* `dns_resolver_address`: Sets a custom address for NetBird's local DNS resolver.

* `external_ip_map`: Sets external IPs maps between local addresses and interfaces.

* `extra_dns_labels`: Sets DNS labels.
