# Operations Center

The Operations Center application includes the latest tagged release of [Operations Center](https://github.com/FuturFusion/operations-center).

At least one trusted client certificate must be provided in the Operations Center seed, otherwise it will be impossible to authenticate to any API endpoint or the web UI post-install.

## Default configuration

If no preseed configuration is provided, Operations Center will start up listening on port 8443 on all network interfaces. Any trusted client certificate provided will be able to authenticate via API or web UI.

## Install seed details

Important seed fields include:

* `trusted_client_certificates`: An array of one or more PEM-encoded client certificates that should be trusted by default.

* `preseed`: A struct referencing various Operations Center system configuration options. For details, please review Operations Center's [API](https://github.com/FuturFusion/operations-center/blob/main/shared/api/system.go).
