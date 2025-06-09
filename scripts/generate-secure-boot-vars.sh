#!/bin/bash

# This is a TEST script to generate TEST secure boot PK/KEK/db variables.
# DON'T let these variables anywhere near a production environment -- we don't want our own PKfail. :)

# Note: There are at least two different tools available in Debian to create and sign the EFI lists,
# efitools and sbsigntool. At the moment, sbsigntool is broken (https://github.com/systemd/systemd/issues/34316#issuecomment-2337311589)
# so we use the commands from efitools.

set -e

OS_NAME="TestOS"
UUID="433f8160-9ab6-4407-8e38-12d70e1d54e5"

mkdir -p certs/efi/

cert-to-efi-sig-list -g "${UUID}" "certs/cas/${OS_NAME}-pk-ca.crt" certs/efi/PK.esl
sign-efi-sig-list -g "${UUID}" -c "certs/cas/${OS_NAME}-pk-ca.crt" -k "certs/cas/${OS_NAME}-pk-ca.key" PK certs/efi/PK.esl certs/efi/PK.auth

cert-to-efi-sig-list -g "${UUID}" "certs/cas/${OS_NAME}-kek-ca1.crt" "certs/efi/${OS_NAME}-kek-ca1.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/cas/${OS_NAME}-kek-ca2.crt" "certs/efi/${OS_NAME}-kek-ca2.esl"
cat "certs/efi/${OS_NAME}-kek-ca1.esl" "certs/efi/${OS_NAME}-kek-ca2.esl" > certs/efi/KEK.esl
sign-efi-sig-list -g "${UUID}" -c "certs/cas/${OS_NAME}-pk-ca.crt" -k "certs/cas/${OS_NAME}-pk-ca.key" KEK certs/efi/KEK.esl certs/efi/KEK.auth

cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-1.crt" "certs/efi/${OS_NAME}-secure-boot-1.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-2.crt" "certs/efi/${OS_NAME}-secure-boot-2.esl"
cert-to-efi-sig-list -g "${UUID}" "certs/${OS_NAME}-secure-boot-3.crt" "certs/efi/${OS_NAME}-secure-boot-3.esl"
cat "certs/efi/${OS_NAME}-secure-boot-1.esl" "certs/efi/${OS_NAME}-secure-boot-2.esl" "certs/efi/${OS_NAME}-secure-boot-3.esl" > certs/efi/db.esl
sign-efi-sig-list -g "${UUID}" -c "certs/cas/${OS_NAME}-kek-ca1.crt" -k "certs/cas/${OS_NAME}-kek-ca1.key" db certs/efi/db.esl certs/efi/db.auth
