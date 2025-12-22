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

for file in $(ls certs/efi/PK*.der); do
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/PK.crt
done

for file in $(ls certs/efi/KEK*.der); do
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/KEK.crt
done

for file in $(ls certs/efi/db_*.der); do
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/db.crt
done

for file in $(ls certs/efi/dbx_*.der); do
    openssl x509 -inform der -in "$file" >> mkosi.images/base/mkosi.extra/usr/lib/incus-osd/certs/dbx.crt || true
done

# To facilitate systemd-sysext verification, symlink db.crt into /usr/lib/verity.d/.

cd mkosi.images/base/mkosi.extra/usr/lib/verity.d/ && ln -s ../incus-osd/certs/db.crt db.crt
