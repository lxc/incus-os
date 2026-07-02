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

# Use two Microsoft db certs to populate dbx with a certificate and various certificate fingerprints
openssl x509 -in "scripts/test/dbx/MicCorUEFCA2011_2011-06-27.crt" -out incus-osd/certs/files/secureboot-DBX-MicCorUEFCA2011_2011-06-27.der -outform DER
openssl x509 -in "scripts/test/dbx/MicWinProPCA2011_2011-10-19.crt" -out incus-osd/certs/files/secureboot-DBX-MicWinProPCA2011_2011-10-19.der -outform DER
cert-to-efi-sig-list -g "${UUID}" "scripts/test/dbx/MicCorUEFCA2011_2011-06-27.crt" "incus-osd/certs/files/DBX-1.esl"
cert-to-efi-hash-list -s 256 -g "${UUID}" "scripts/test/dbx/MicWinProPCA2011_2011-10-19.crt" "incus-osd/certs/files/DBX-2a.esl"
cert-to-efi-hash-list -s 384 -g "${UUID}" "scripts/test/dbx/MicWinProPCA2011_2011-10-19.crt" "incus-osd/certs/files/DBX-2b.esl"
cert-to-efi-hash-list -s 512 -g "${UUID}" "scripts/test/dbx/MicWinProPCA2011_2011-10-19.crt" "incus-osd/certs/files/DBX-2c.esl"

# Also include the SHA256 hash of a random EFI binary that should be prohibited from running
cat "incus-osd/certs/files/DBX-1.esl" "incus-osd/certs/files/DBX-2a.esl" "incus-osd/certs/files/DBX-2b.esl" "incus-osd/certs/files/DBX-2c.esl" "scripts/test/dbx/revoked-efi-binary.esl" > incus-osd/certs/files/DBX.esl
sign-efi-sig-list -g "${UUID}" -c "incus-osd/certs/files/secureboot-KEK-R1.crt" -k "certs/secureboot-KEK-R1.key" dbx incus-osd/certs/files/DBX.esl incus-osd/certs/files/DBX.auth

find incus-osd/certs/files/ -name '*.esl' -delete

mkdir -p certs/efi/updates/

# Prepare a db update
FINGERPRINT=$(openssl x509 -in "incus-osd/certs/files/secureboot-DB-3-R1.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-DB-3-R1.crt" "certs/efi/updates/db_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "incus-osd/certs/files/secureboot-KEK-R1.crt" -k "certs/secureboot-KEK-R1.key" db "certs/efi/updates/db_${FINGERPRINT}.esl" "certs/efi/updates/db_${FINGERPRINT}.auth"

# Prepare a dbx certificate update
FINGERPRINT=$(openssl x509 -in "incus-osd/certs/files/secureboot-DBX-1-R1.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "incus-osd/certs/files/secureboot-DBX-1-R1.crt" "certs/efi/updates/dbx_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "incus-osd/certs/files/secureboot-KEK-R1.crt" -k "certs/secureboot-KEK-R1.key" dbx "certs/efi/updates/dbx_${FINGERPRINT}.esl" "certs/efi/updates/dbx_${FINGERPRINT}.auth"
