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

# PK CA
cert-to-efi-sig-list -g "${UUID}" "certs/cas/${OS_NAME}-pk-ca.crt" certs/efi/PK.esl
sign-efi-sig-list -g "${UUID}" -c "certs/cas/${OS_NAME}-pk-ca.crt" -k "certs/cas/${OS_NAME}-pk-ca.key" PK certs/efi/PK.esl certs/efi/PK.auth

# KEK CAs
cert-to-efi-sig-list -g "${UUID}" "certs/cas/${OS_NAME}-kek-ca1.crt" "certs/efi/${OS_NAME}-kek-ca1.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/cas/${OS_NAME}-kek-ca2.crt" "certs/efi/${OS_NAME}-kek-ca2.esl"
cat "certs/efi/${OS_NAME}-kek-ca1.esl" "certs/efi/${OS_NAME}-kek-ca2.esl" > certs/efi/KEK.esl
sign-efi-sig-list -g "${UUID}" -c "certs/cas/${OS_NAME}-pk-ca.crt" -k "certs/cas/${OS_NAME}-pk-ca.key" KEK certs/efi/KEK.esl certs/efi/KEK.auth

# First two trusted secure boot keys
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-1.crt" "certs/efi/${OS_NAME}-secure-boot-1.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-2.crt" "certs/efi/${OS_NAME}-secure-boot-2.esl"
cat "certs/efi/${OS_NAME}-secure-boot-1.esl" "certs/efi/${OS_NAME}-secure-boot-2.esl" > certs/efi/db.esl
sign-efi-sig-list -g "${UUID}" -c "certs/cas/${OS_NAME}-kek-ca1.crt" -k "certs/cas/${OS_NAME}-kek-ca1.key" db certs/efi/db.esl certs/efi/db.auth

mkdir -p certs/efi/updates/

# Prepare a db update
FINGERPRINT=$(openssl x509 -in "certs/${OS_NAME}-secure-boot-3.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-3.crt" "certs/efi/updates/db_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "certs/cas/${OS_NAME}-kek-ca1.crt" -k "certs/cas/${OS_NAME}-kek-ca1.key" db "certs/efi/updates/db_${FINGERPRINT}.esl" "certs/efi/updates/db_${FINGERPRINT}.auth"

# Prepare a dbx update
FINGERPRINT=$(openssl x509 -in "certs/${OS_NAME}-secure-boot-4.crt" -noout -fingerprint -sha256 | cut -d '=' -f2 | tr -d ':')
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-4.crt" "certs/efi/updates/dbx_${FINGERPRINT}.esl"
sign-efi-sig-list -g "${UUID}" -a -c "certs/cas/${OS_NAME}-kek-ca1.crt" -k "certs/cas/${OS_NAME}-kek-ca1.key" dbx "certs/efi/updates/dbx_${FINGERPRINT}.esl" "certs/efi/updates/dbx_${FINGERPRINT}.auth"
