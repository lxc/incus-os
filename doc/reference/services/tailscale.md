# Tailscale

The [Tailscale](https://tailscale.com/) service allows configuring a Tailscale VPN client.

## Configuration options

The full API structs for the service can be viewed [online](https://github.com/lxc/incus-os/blob/main/incus-osd/api/service_tailscale.go).

The following configuration options can be set:

* `enabled`: If `true`, enable the Tailscale service.

* `login_server`: The Tailscale login server.

* `auth_key`: A Tailscale authentication key.

* `accept_routes`: If `true`, accept routes.

* `advertised_routes`: An array of routes to advertise.
