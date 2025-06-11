#!/bin/bash

# This is a TEST script to switch a TEST secure boot signing key.

set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <cert number>"
    exit 1
fi

OS_NAME="TestOS"

rm ./mkosi.crt ./mkosi.key ./mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der

ln -s "./certs/${OS_NAME}-secure-boot-$1.crt" ./mkosi.crt
ln -s "./certs/${OS_NAME}-secure-boot-$1.key" ./mkosi.key
openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER
