#!/bin/sh

# This is a TEST script to generate TEST secure boot PK/KEK/db variable updates.
# DON'T let these variables anywhere near a production environment -- we don't want our own PKfail. :)

# Note: There are at least two different tools available in Debian to create and sign the EFI lists,
# efitools and sbsigntool. At the moment, sbsigntool is broken (https://github.com/systemd/systemd/issues/34316#issuecomment-2337311589)
# so we use the commands from efitools.

set -e

OS_NAME="TestOS"
UUID="433f8160-9ab6-4407-8e38-12d70e1d54e5"

if [ -d certs/efi/ ]; then
    echo "Test secure boot signed keys already appear to have been generated, exiting."
    exit 0
fi

mkdir -p certs/efi/

# PK
openssl x509 -in "certs/${OS_NAME}-secureboot-PK-R1.crt" -out certs/efi/PK.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-PK-R1.crt" certs/efi/PK.esl
sign-efi-sig-list -g "${UUID}" -c "certs/${OS_NAME}-secureboot-PK-R1.crt" -k "certs/${OS_NAME}-secureboot-PK-R1.key" PK certs/efi/PK.esl certs/efi/PK.auth

# KEKs
openssl x509 -in "certs/${OS_NAME}-secureboot-KEK-R1.crt" -out certs/efi/KEK_1.der -outform DER
openssl x509 -in "certs/${OS_NAME}-secureboot-KEK-R2.crt" -out certs/efi/KEK_2.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-KEK-R1.crt" "certs/efi/${OS_NAME}-kek-1.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-KEK-R2.crt" "certs/efi/${OS_NAME}-kek-2.esl"
cat "certs/efi/${OS_NAME}-kek-1.esl" "certs/efi/${OS_NAME}-kek-2.esl" > certs/efi/KEK.esl
sign-efi-sig-list -g "${UUID}" -c "certs/${OS_NAME}-secureboot-PK-R1.crt" -k "certs/${OS_NAME}-secureboot-PK-R1.key" KEK certs/efi/KEK.esl certs/efi/KEK.auth

# First two trusted secure boot keys
openssl x509 -in "certs/${OS_NAME}-secureboot-1-R1.crt" -out certs/efi/db_1.der -outform DER
openssl x509 -in "certs/${OS_NAME}-secureboot-2-R1.crt" -out certs/efi/db_2.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-1-R1.crt" "certs/efi/${OS_NAME}-secureboot-1.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-2-R1.crt" "certs/efi/${OS_NAME}-secureboot-2.esl"
cat "certs/efi/${OS_NAME}-secureboot-1.esl" "certs/efi/${OS_NAME}-secureboot-2.esl" > certs/efi/db.esl
sign-efi-sig-list -g "${UUID}" -c "certs/${OS_NAME}-secureboot-KEK-R1.crt" -k "certs/${OS_NAME}-secureboot-KEK-R1.key" db certs/efi/db.esl certs/efi/db.auth

mkdir -p certs/efi/updates/

# Prepare a db update
FINGERPRINT=$(openssl x509 -in "certs/${OS_NAME}-secureboot-3-R1.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-3-R1.crt" "certs/efi/updates/db_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "certs/${OS_NAME}-secureboot-KEK-R1.crt" -k "certs/${OS_NAME}-secureboot-KEK-R1.key" db "certs/efi/updates/db_${FINGERPRINT}.esl" "certs/efi/updates/db_${FINGERPRINT}.auth"

# Prepare a dbx update
FINGERPRINT=$(openssl x509 -in "certs/${OS_NAME}-secureboot-4-R1.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secureboot-4-R1.crt" "certs/efi/updates/dbx_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "certs/${OS_NAME}-secureboot-KEK-R1.crt" -k "certs/${OS_NAME}-secureboot-KEK-R1.key" dbx "certs/efi/updates/dbx_${FINGERPRINT}.esl" "certs/efi/updates/dbx_${FINGERPRINT}.auth"
