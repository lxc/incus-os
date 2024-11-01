#!/bin/sh -eux

# Copy the certificate store to /usr.
mkdir -p "${DESTDIR}/usr/share/certs"
cp /etc/ssl/certs/ca-certificates.crt "${DESTDIR}/usr/share/certs/"

exit 0
