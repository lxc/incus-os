#!/bin/bash

set -e

### Versions ###
INCUS_TERRAFORM_VERSION=0.5.0
MIGRATION_MANAGER_VERSION=main
KPX_VERSION=1.12.1
OPENTOFU_VERSION=1.10.6
OPERATIONS_CENTER_VERSION=main

### Incus terraform/tofu plugin ###
if [ -d terraform-provider-incus ]; then
    pushd terraform-provider-incus
    git reset --hard && git fetch --depth 1 origin "v${INCUS_TERRAFORM_VERSION}:refs/tags/v${INCUS_TERRAFORM_VERSION}" && git checkout "v${INCUS_TERRAFORM_VERSION}"
    popd
else
    git clone https://github.com/lxc/terraform-provider-incus.git terraform-provider-incus --depth 1 -b "v${INCUS_TERRAFORM_VERSION}"
fi

pushd terraform-provider-incus
go build .
strip terraform-provider-incus

# tofu expects the provider to be in a versioned path. To make the copy logic easier in the Makefile, construct the versioned path here.
ARCH=$(uname -m)
if [ "${ARCH}" = "aarch64" ]; then
    ARCH="arm64"
elif [ "${ARCH}" = "x86_64" ]; then
    ARCH="amd64"
else
    die "Unsupported architecture: ${ARCH}"
fi

rm -rf lxc/
mkdir -p "lxc/incus/${INCUS_TERRAFORM_VERSION}/linux_${ARCH}"
mv terraform-provider-incus "lxc/incus/${INCUS_TERRAFORM_VERSION}/linux_${ARCH}/terraform-provider-incus_v${INCUS_TERRAFORM_VERSION}"
popd

### kpx ###
if [ -d kpx ]; then
    pushd kpx
    git reset --hard && git fetch --depth 1 origin "v${KPX_VERSION}:refs/tags/v${KPX_VERSION}" && git checkout "v${KPX_VERSION}"
    popd
else
    git clone https://github.com/momiji/kpx.git kpx --depth 1 -b "v${KPX_VERSION}"
fi

pushd kpx
patch -p1 < ../../patches/kpx-0001-Enable-IPv6-support.patch

go build -o kpx -ldflags="s -w -X github.com/momiji/kpx.AppVersion=${KPX_VERSION}" ./cli
strip kpx
popd

### Migration Manager ###
if [ -d migration-manager ]; then
    pushd migration-manager
    git reset --hard && git pull
    popd
else
    git clone https://github.com/FuturFusion/migration-manager.git migration-manager --depth 1 -b "${MIGRATION_MANAGER_VERSION}"
fi

pushd migration-manager
go build -o migration-managerd ./cmd/migration-managerd
go build -o migration-manager-worker ./cmd/migration-manager-worker
strip migration-managerd migration-manager-worker

pushd ui
YARN_ENABLE_HARDENED_MODE=0 YARN_ENABLE_IMMUTABLE_INSTALLS=false yarnpkg install && yarnpkg build
popd

popd

### opentofu ###
if [ -d opentofu ]; then
    pushd opentofu
    git reset --hard && git fetch --depth 1 origin "v${OPENTOFU_VERSION}:refs/tags/v${OPENTOFU_VERSION}" && git checkout "v${OPENTOFU_VERSION}"
    popd
else
    git clone https://github.com/opentofu/opentofu.git opentofu --depth 1 -b "v${OPENTOFU_VERSION}"
fi

pushd opentofu
go build -o tofu ./cmd/tofu
strip tofu
popd

### Operations Center ###
if [ -d operations-center ]; then
    pushd operations-center
    git reset --hard && git pull
    popd
else
    git clone https://github.com/FuturFusion/operations-center.git operations-center --depth 1 -b "${OPERATIONS_CENTER_VERSION}"
fi

pushd operations-center
go build -o operations-centerd ./cmd/operations-centerd
strip operations-centerd

pushd ui
YARN_ENABLE_HARDENED_MODE=0 YARN_ENABLE_IMMUTABLE_INSTALLS=false yarnpkg install && yarnpkg build
popd

popd
