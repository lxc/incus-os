#!/bin/sh

rm -rf mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/
rm -rf mkosi.images/base/mkosi.extra/usr/lib/verity.d/
mkdir -p mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/
mkdir -p mkosi.images/base/mkosi.extra/usr/lib/verity.d/

# Copy the IncusOS CAs to /usr/lib/incus-osd/certs/

cp certs/cas/root-ca.crt mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/
cp certs/cas/secureboot-ca.crt mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/

# To support systems running without Secure Boot enabled, we bake a copy of the Secure Boot
# PEM-encoded certificates into the base image under /usr/lib/incus-osd/certs/.

for file in certs/efi/secureboot-PK-*.der; do
    [ -e "$file" ] || break
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/PK.crt
done

for file in certs/efi/secureboot-KEK-*.der; do
    [ -e "$file" ] || break
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/KEK.crt
done

for file in certs/efi/secureboot-DB-*.der; do
    [ -e "$file" ] || break
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/db.crt

    # To facilitate systemd-sysext verification, copy each certificate into /usr/lib/verity.d/.
    # Each certificate must be in its own file, as systemd-sysext doesn't seem to support multiple certificates in one file.
    cert=$(basename "$file")
    openssl x509 -inform der -in "$file" -out "mkosi.images/base/mkosi.extra/usr/lib/verity.d/${cert%.der}.crt"
done

for file in certs/efi/secureboot-DBX-*.der; do
    [ -e "$file" ] || break
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/dbx.crt || true
done

# Copy certs into the initrd package source for boot PE binary verification.
rm -rf mkosi.packages/incusos-initrd-utils/certs/
cp -r mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/ mkosi.packages/incusos-initrd-utils/
