#!/bin/sh

# This is a TEST script to generate TEST secure boot certificates and corresponding CAs.
# DON'T let these certificates anywhere near a production environment -- we don't want our own PKfail. :)

set -e

OS_NAME="TestOS"

if [ -d certs/ ]; then
    echo "Test certificates already appear to have been generated, exiting."
    exit 0
fi

find incus-osd/certs/files/ -name '*.auth' -delete
find incus-osd/certs/files/ -name '*.crt' -delete
find incus-osd/certs/files/ -name '*.der' -delete

mkdir -p certs/cas/
cat <<EOF > certs/cas/ssl.conf
[ v3_ca ]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, cRLSign, keyCertSign
EOF

# Root CA
openssl ecparam -genkey -name prime256v1 -out "certs/cas/root-E1.key"
openssl req -x509 -new -extensions v3_ca -SHA384 -nodes -key "certs/cas/root-E1.key" -days 3650 -out "incus-osd/certs/files/root-E1.crt" -subj "/CN=${OS_NAME} - Root E1/O=${OS_NAME}"

# Secure Boot CA
openssl ecparam -genkey -name prime256v1 -out "certs/cas/secureboot-E1.key"
openssl req -new -SHA384 -key "certs/cas/secureboot-E1.key" -nodes -out "certs/cas/secureboot-E1.csr" -subj "/CN=${OS_NAME} - Secure Boot E1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA384 -days 3650 -in "certs/cas/secureboot-E1.csr" -CA "incus-osd/certs/files/root-E1.crt" -CAkey "certs/cas/root-E1.key" -out "incus-osd/certs/files/secureboot-E1.crt"

# Update CA
openssl ecparam -genkey -name prime256v1 -out "certs/cas/update-E1.key"
openssl req -new -SHA384 -key "certs/cas/update-E1.key" -nodes -out "certs/cas/update-E1.csr" -subj "/CN=${OS_NAME} - Update E1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA384 -days 3650 -in "certs/cas/update-E1.csr" -CA "incus-osd/certs/files/root-E1.crt" -CAkey "certs/cas/root-E1.key" -out "incus-osd/certs/files/update-E1.crt"

# PK
openssl genrsa -out "certs/secureboot-PK-R1.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-PK-R1.key" -nodes -out "certs/secureboot-PK-R1.csr" -subj "/CN=${OS_NAME} - Secure Boot PK R1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-PK-R1.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-PK-R1.crt"

# KEKs
openssl genrsa -out "certs/secureboot-KEK-R1.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-KEK-R1.key" -nodes -out "certs/secureboot-KEK-R1.csr" -subj "/CN=${OS_NAME} - Secure Boot KEK R1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-KEK-R1.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-KEK-R1.crt"

openssl genrsa -out "certs/secureboot-KEK-R2.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-KEK-R2.key" -nodes -out "certs/secureboot-KEK-R2.csr" -subj "/CN=${OS_NAME} - Secure Boot KEK R2/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-KEK-R2.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-KEK-R2.crt"

# Secure Boot keys
openssl genrsa -out "certs/secureboot-DB-1-R1.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-DB-1-R1.key" -nodes -out "certs/secureboot-DB-1-R1.csr" -subj "/CN=${OS_NAME} - Secure Boot 1 R1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-DB-1-R1.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-DB-1-R1.crt"

openssl genrsa -out "certs/secureboot-DB-2-R1.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-DB-2-R1.key" -nodes -out "certs/secureboot-DB-2-R1.csr" -subj "/CN=${OS_NAME} - Secure Boot 2 R1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-DB-2-R1.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-DB-2-R1.crt"

openssl genrsa -out "certs/secureboot-DB-3-R1.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-DB-3-R1.key" -nodes -out "certs/secureboot-DB-3-R1.csr" -subj "/CN=${OS_NAME} - Secure Boot 3 R1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-DB-3-R1.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-DB-3-R1.crt"

openssl genrsa -out "certs/secureboot-DBX-4-R1.key" 2048
openssl req -new -SHA256 -key "certs/secureboot-DBX-4-R1.key" -nodes -out "certs/secureboot-DBX-4-R1.csr" -subj "/CN=${OS_NAME} - Secure Boot 4 R1/O=${OS_NAME}"
openssl x509 -req -extensions v3_ca -extfile certs/cas/ssl.conf -SHA256 -days 3650 -in "certs/secureboot-DBX-4-R1.csr" -CA "incus-osd/certs/files/secureboot-E1.crt" -CAkey "certs/cas/secureboot-E1.key" -out "incus-osd/certs/files/secureboot-DBX-4-R1.crt"

find certs/ -name '*.csr' -delete
