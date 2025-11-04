# Getting an image
ISO and raw images are distributed via the [Linux Containers {abbr}`CDN (Content Delivery Network)`](https://images.linuxcontainers.org/os/).

IncusOS doesn't feature a traditional installer, and relies on an [installation seed](../reference/seed.md)
to provide configuration details and defaults during install. This install
seed can be manually crafted, or you can use either of the two utilities
described below to automate the process.

There are two more user-friendly methods to get an IncusOS install image:
- A web-based customization tool
- A command line flasher tool

In either case, you will need to provide an initial trusted client certificate.

You can get yours by running

    incus remote get-client-certificate

## IncusOS customizer

The web-based [IncusOS customizer](https://incusos-customizer.linuxcontainers.org/ui/)
is the most user-friendly way to get an IncusOS install image. The web page
will let you make a few simple selections, then directly download an install
image that's ready for immediate use.

## Flasher tool

The flasher tool is provided for more advanced users who need
to perform more customizations of the install seed than the web-based customizer
supports.

It can be built and run on a system with the Go compiler installed using:

    go install github.com/lxc/incus-os/incus-osd/cmd/flasher-tool@latest
    flasher-tool

when run, you will first be prompted for the image format you want to use, either ISO
(default) or raw disk image. Note that the ISO isn't a hybrid image; if you
want to boot from a USB stick you should choose the raw disk image format.

The flasher tool will then connect to the Linux Containers CDN and download the
latest release.

Once downloaded, you will be presented with an interactive menu you can use to
customize the install options.

To get your certificate trusted by Incus during installation, you'll
have to provide an Incus seed like this, substituting your certificate:

```
apply_defaults: true
preseed:
    certificates:
        - name: demo
          type: client
          certificate: |
            -----BEGIN CERTIFICATE-----
            MIIB4TCCAWagAwIBAgIQVrBNb+LgEvX/aDNNOLM2iTAKBggqhkjOPQQDAzA4MRkw
            FwYDVQQKExBMaW51eCBDb250YWluZXJzMRswGQYDVQQDDBJnaWJtYXRAZnV0dXJm
            dXNpb24wHhcNMjUwNjA1MTgwODAwWhcNMzUwNjAzMTgwODAwWjA4MRkwFwYDVQQK
            ExBMaW51eCBDb250YWluZXJzMRswGQYDVQQDDBJnaWJtYXRAZnV0dXJmdXNpb24w
            djAQBgcqhkjOPQIBBgUrgQQAIgNiAAS8Tsj87gyhkR6gUoTa9dooWhwApI9MlsZS
            M9HkNdgLG+0d2yU3JXru4AbCD+pslsL5mnSjbmF7BhqSAT0opQtyFMfB7hrCJkVB
            nnebLNOqzrOVnxYqnD1HnfKo6RVmXpGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNV
            HSUEDDAKBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMAoGCCqGSM49BAMDA2kAMGYC
            MQC/Y4nAuV09z/zeh0aN+XV+kI9WLnITFprSHREIaES3r49cTkpoV8wFCwdLjbSb
            NwECMQCx5H/H3hyXJen3uLbqRxTzw5jjx1M4dO4fru+VmoOKmTSmKVq3r2j449iD
            GrzY7EQ=
            -----END CERTIFICATE-----
```

After writing the image and exiting, you can then install IncusOS from the
resulting image.
