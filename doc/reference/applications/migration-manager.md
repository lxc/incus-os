# Migration Manager

The Migration Manager application includes the latest tagged release of [Migration Manager](https://github.com/FuturFusion/migration-manager).

At least one trusted client certificate must be provided in the Migration Manger seed, otherwise it will be impossible to authenticate to any API endpoint or the web UI post-install.

## Default configuration

If no preseed configuration is provided, Migration Manager will start up listening on port 8443 on all network interfaces. Any trusted client certificate provided will be able to authenticate via API or web UI.

## Install seed details

Important seed fields include:

* `trusted_client_certificates`: An array of one or more PEM-encoded client certificates that should be trusted by default.

* `preseed`: A struct referencing various Migration Manager system configuration options. For details, please review Migration Manager's [API](https://github.com/FuturFusion/migration-manager/blob/main/shared/api/system.go).
