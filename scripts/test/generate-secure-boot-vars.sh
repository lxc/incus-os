#!/bin/sh

# This is a TEST script to generate TEST secure boot PK/KEK/db variable updates.
# DON'T let these variables anywhere near a production environment -- we don't want our own PKfail. :)

# Note: There are at least two different tools available in Debian to create and sign the EFI lists,
# efitools and sbsigntool. At the moment, sbsigntool is broken (https://github.com/systemd/systemd/issues/34316#issuecomment-2337311589)
# so we use the commands from efitools.

set -e

UUID="433f8160-9ab6-4407-8e38-12d70e1d54e5"

if [ -d certs/efi/updates/ ]; then
    echo "Test secure boot signed keys already appear to have been generated, exiting."
    exit 0
fi

# PK
openssl x509 -in "incus-osd/certs/files/secureboot-PK-R1.crt" -out incus-osd/certs/files/secureboot-PK-R1.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-PK-R1.crt" incus-osd/certs/files/PK.esl
sign-efi-sig-list -g "${UUID}" -c "incus-osd/certs/files/secureboot-PK-R1.crt" -k "certs/secureboot-PK-R1.key" PK incus-osd/certs/files/PK.esl incus-osd/certs/files/PK.auth

# KEKs
openssl x509 -in "incus-osd/certs/files/secureboot-KEK-R1.crt" -out incus-osd/certs/files/secureboot-KEK-R1.der -outform DER
openssl x509 -in "incus-osd/certs/files/secureboot-KEK-R2.crt" -out incus-osd/certs/files/secureboot-KEK-R2.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-KEK-R1.crt" "incus-osd/certs/files/KEK-1.esl"
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-KEK-R2.crt" "incus-osd/certs/files/KEK-2.esl"
cat "incus-osd/certs/files/KEK-1.esl" "incus-osd/certs/files/KEK-2.esl" > incus-osd/certs/files/KEK.esl
sign-efi-sig-list -g "${UUID}" -c "incus-osd/certs/files/secureboot-PK-R1.crt" -k "certs/secureboot-PK-R1.key" KEK incus-osd/certs/files/KEK.esl incus-osd/certs/files/KEK.auth

# First two trusted secure boot keys
openssl x509 -in "incus-osd/certs/files/secureboot-DB-1-R1.crt" -out incus-osd/certs/files/secureboot-DB-1-R1.der -outform DER
openssl x509 -in "incus-osd/certs/files/secureboot-DB-2-R1.crt" -out incus-osd/certs/files/secureboot-DB-2-R1.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-DB-1-R1.crt" "incus-osd/certs/files/DB-1.esl"
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-DB-2-R1.crt" "incus-osd/certs/files/DB-2.esl"
cat "incus-osd/certs/files/DB-1.esl" "incus-osd/certs/files/DB-2.esl" > incus-osd/certs/files/DB.esl
sign-efi-sig-list -g "${UUID}" -c "incus-osd/certs/files/secureboot-KEK-R1.crt" -k "certs/secureboot-KEK-R1.key" db incus-osd/certs/files/DB.esl incus-osd/certs/files/DB.auth

find incus-osd/certs/files/ -name '*.esl' -delete

mkdir -p certs/efi/updates/

# Prepare a db update
FINGERPRINT=$(openssl x509 -in "incus-osd/certs/files/secureboot-DB-3-R1.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-DB-3-R1.crt" "certs/efi/updates/db_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "incus-osd/certs/files/secureboot-KEK-R1.crt" -k "certs/secureboot-KEK-R1.key" db "certs/efi/updates/db_${FINGERPRINT}.esl" "certs/efi/updates/db_${FINGERPRINT}.auth"

# Prepare a dbx update
FINGERPRINT=$(openssl x509 -in "incus-osd/certs/files/secureboot-DBX-4-R1.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-DBX-4-R1.crt" "certs/efi/updates/dbx_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "incus-osd/certs/files/secureboot-KEK-R1.crt" -k "certs/secureboot-KEK-R1.key" dbx "certs/efi/updates/dbx_${FINGERPRINT}.esl" "certs/efi/updates/dbx_${FINGERPRINT}.auth"
