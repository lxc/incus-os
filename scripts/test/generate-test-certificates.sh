#!/bin/sh

# This is a TEST script to generate TEST secure boot certificates and corresponding CAs.
# DON'T let these certificates anywhere near a production environment -- we don't want our own PKfail. :)

set -e

OS_NAME="TestOS"

if [ -d certs/ ]; then
    echo "Test certificates already appear to have been generated, exiting."
    exit 0
fi

mkdir -p certs/cas/
cat <<EOF > certs/cas/ssl.conf
[ v3_ca ]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, cRLSign, keyCertSign
EOF

# Root CA
openssl ecparam -genkey -name prime256v1 -out "certs/cas/${OS_NAME}-root-ca.key"
openssl req -x509 -new -extensions v3_ca -SHA384 -nodes -key "certs/cas/${OS_NAME}-root-ca.key" -days 3650 -out "certs/cas/${OS_NAME}-root-ca.crt" -subj "/CN=${OS_NAME} Root CA/O=${OS_NAME}"

# PK CA
openssl genrsa -out "certs/cas/${OS_NAME}-pk-ca.key" 2048
openssl req -new -SHA256 -key "certs/cas/${OS_NAME}-pk-ca.key" -nodes -out "certs/cas/${OS_NAME}-pk-ca.csr" -subj "/CN=${OS_NAME} PK CA/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/cas/${OS_NAME}-pk-ca.csr" -CA "certs/cas/${OS_NAME}-root-ca.crt" -CAkey "certs/cas/${OS_NAME}-root-ca.key" -out "certs/cas/${OS_NAME}-pk-ca.crt"

# KEK CAs
openssl genrsa -out "certs/cas/${OS_NAME}-kek-ca1.key" 2048
openssl req -new -SHA256 -key "certs/cas/${OS_NAME}-kek-ca1.key" -nodes -out "certs/cas/${OS_NAME}-kek-ca1.csr" -subj "/CN=${OS_NAME} KEK CA1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/cas/${OS_NAME}-kek-ca1.csr" -CA "certs/cas/${OS_NAME}-pk-ca.crt" -CAkey "certs/cas/${OS_NAME}-pk-ca.key" -out "certs/cas/${OS_NAME}-kek-ca1.crt"

openssl genrsa -out "certs/cas/${OS_NAME}-kek-ca2.key" 2048
openssl req -new -SHA256 -key "certs/cas/${OS_NAME}-kek-ca2.key" -nodes -out "certs/cas/${OS_NAME}-kek-ca2.csr" -subj "/CN=${OS_NAME} KEK CA2/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/cas/${OS_NAME}-kek-ca2.csr" -CA "certs/cas/${OS_NAME}-pk-ca.crt" -CAkey "certs/cas/${OS_NAME}-pk-ca.key" -out "certs/cas/${OS_NAME}-kek-ca2.crt"

# Secure Boot keys
openssl genrsa -out "certs/${OS_NAME}-secure-boot-1.key" 2048
openssl req -new -SHA256 -key "certs/${OS_NAME}-secure-boot-1.key" -nodes -out "certs/${OS_NAME}-secure-boot-1.csr" -subj "/CN=${OS_NAME} Secure Boot Key 1/O=${OS_NAME}"
openssl x509 -req -SHA256 -days 365 -in "certs/${OS_NAME}-secure-boot-1.csr" -CA "certs/cas/${OS_NAME}-kek-ca1.crt" -CAkey "certs/cas/${OS_NAME}-kek-ca1.key" -out "certs/${OS_NAME}-secure-boot-1.crt"

openssl genrsa -out "certs/${OS_NAME}-secure-boot-2.key" 2048
openssl req -new -SHA256 -key "certs/${OS_NAME}-secure-boot-2.key" -nodes -out "certs/${OS_NAME}-secure-boot-2.csr" -subj "/CN=${OS_NAME} Secure Boot Key 2/O=${OS_NAME}"
openssl x509 -req -SHA256 -days 365 -in "certs/${OS_NAME}-secure-boot-2.csr" -CA "certs/cas/${OS_NAME}-kek-ca1.crt" -CAkey "certs/cas/${OS_NAME}-kek-ca1.key" -out "certs/${OS_NAME}-secure-boot-2.crt"

openssl genrsa -out "certs/${OS_NAME}-secure-boot-3.key" 2048
openssl req -new -SHA256 -key "certs/${OS_NAME}-secure-boot-3.key" -nodes -out "certs/${OS_NAME}-secure-boot-3.csr" -subj "/CN=${OS_NAME} Secure Boot Key 3/O=${OS_NAME}"
openssl x509 -req -SHA256 -days 365 -in "certs/${OS_NAME}-secure-boot-3.csr" -CA "certs/cas/${OS_NAME}-kek-ca1.crt" -CAkey "certs/cas/${OS_NAME}-kek-ca1.key" -out "certs/${OS_NAME}-secure-boot-3.crt"

openssl genrsa -out "certs/${OS_NAME}-secure-boot-4.key" 2048
openssl req -new -SHA256 -key "certs/${OS_NAME}-secure-boot-4.key" -nodes -out "certs/${OS_NAME}-secure-boot-4.csr" -subj "/CN=${OS_NAME} Secure Boot Key 4/O=${OS_NAME}"
openssl x509 -req -SHA256 -days 365 -in "certs/${OS_NAME}-secure-boot-4.csr" -CA "certs/cas/${OS_NAME}-kek-ca1.crt" -CAkey "certs/cas/${OS_NAME}-kek-ca1.key" -out "certs/${OS_NAME}-secure-boot-4.crt"
