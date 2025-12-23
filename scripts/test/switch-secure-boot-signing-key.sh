#!/bin/sh

# This is a TEST script to switch a TEST secure boot signing key.

set -e

OS_NAME="TestOS"

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <cert number>"
    exit 1
fi

if [ ! -d certs/ ]; then
    echo "Directory './certs/' doesn't exist, exiting"
    exit 1
fi

rm -f ./mkosi.crt ./mkosi.key ./mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der

# mkosi seems to have several hard-coded assumptions that the secure boot key will always be called "mkosi.{crt,key}".
ln -s "./certs/${OS_NAME}-secureboot-$1-R1.crt" ./mkosi.crt
ln -s "./certs/${OS_NAME}-secureboot-$1-R1.key" ./mkosi.key

mkdir -p mkosi.images/base/mkosi.extra/boot/EFI/
openssl x509 -in mkosi.crt -out mkosi.images/base/mkosi.extra/boot/EFI/mkosi.der -outform DER
