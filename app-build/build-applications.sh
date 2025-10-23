#!/bin/bash

set -e

### Versions ###
HASHICORP_TERRAFORM_NULL_VERSION=3.2.4
HASHICORP_TERRAFORM_RANDOM_VERSION=3.7.2
HASHICORP_TERRAFORM_TIME_VERSION=0.13.1
INCUS_TERRAFORM_VERSION=0.5.1
MIGRATION_MANAGER_VERSION=main
KPX_VERSION=1.12.2
OPENTOFU_VERSION=1.10.6
OPERATIONS_CENTER_VERSION=main
TAILSCALE_VERSION=1.90.1

### Detect architecture string for tofu providers ###
ARCH=$(uname -m)
if [ "${ARCH}" = "aarch64" ]; then
    ARCH="arm64"
elif [ "${ARCH}" = "x86_64" ]; then
    ARCH="amd64"
else
    die "Unsupported architecture: ${ARCH}"
fi

### Hashicorp terraform/tofu null plugin ###
if [ -d terraform-provider-null ]; then
    pushd terraform-provider-null
    git reset --hard && git fetch --depth 1 origin "v${HASHICORP_TERRAFORM_NULL_VERSION}:refs/tags/v${HASHICORP_TERRAFORM_NULL_VERSION}" && git checkout "v${HASHICORP_TERRAFORM_NULL_VERSION}"
    popd
else
    git clone https://github.com/hashicorp/terraform-provider-null.git terraform-provider-null --depth 1 -b "v${HASHICORP_TERRAFORM_NULL_VERSION}"
fi

pushd terraform-provider-null
go build .
strip terraform-provider-null

# tofu expects the provider to be in a versioned path. To make the copy logic easier in the Makefile, construct the versioned path here.
rm -rf hashicorp/
mkdir -p "hashicorp/null/${HASHICORP_TERRAFORM_NULL_VERSION}/linux_${ARCH}"
mv terraform-provider-null "hashicorp/null/${HASHICORP_TERRAFORM_NULL_VERSION}/linux_${ARCH}/terraform-provider-null_v${HASHICORP_TERRAFORM_NULL_VERSION}"
popd

### Hashicorp terraform/tofu random plugin ###
if [ -d terraform-provider-random ]; then
    pushd terraform-provider-random
    git reset --hard && git fetch --depth 1 origin "v${HASHICORP_TERRAFORM_RANDOM_VERSION}:refs/tags/v${HASHICORP_TERRAFORM_RANDOM_VERSION}" && git checkout "v${HASHICORP_TERRAFORM_RANDOM_VERSION}"
    popd
else
    git clone https://github.com/hashicorp/terraform-provider-random.git terraform-provider-random --depth 1 -b "v${HASHICORP_TERRAFORM_RANDOM_VERSION}"
fi

pushd terraform-provider-random
go build .
strip terraform-provider-random

# tofu expects the provider to be in a versioned path. To make the copy logic easier in the Makefile, construct the versioned path here.
rm -rf hashicorp/
mkdir -p "hashicorp/random/${HASHICORP_TERRAFORM_RANDOM_VERSION}/linux_${ARCH}"
mv terraform-provider-random "hashicorp/random/${HASHICORP_TERRAFORM_RANDOM_VERSION}/linux_${ARCH}/terraform-provider-random_v${HASHICORP_TERRAFORM_RANDOM_VERSION}"
popd

### Hashicorp terraform/tofu time plugin ###
if [ -d terraform-provider-time ]; then
    pushd terraform-provider-time
    git reset --hard && git fetch --depth 1 origin "v${HASHICORP_TERRAFORM_TIME_VERSION}:refs/tags/v${HASHICORP_TERRAFORM_TIME_VERSION}" && git checkout "v${HASHICORP_TERRAFORM_TIME_VERSION}"
    popd
else
    git clone https://github.com/hashicorp/terraform-provider-time.git terraform-provider-time --depth 1 -b "v${HASHICORP_TERRAFORM_TIME_VERSION}"
fi

pushd terraform-provider-time
go build .
strip terraform-provider-time

# tofu expects the provider to be in a versioned path. To make the copy logic easier in the Makefile, construct the versioned path here.
rm -rf hashicorp/
mkdir -p "hashicorp/time/${HASHICORP_TERRAFORM_TIME_VERSION}/linux_${ARCH}"
mv terraform-provider-time "hashicorp/time/${HASHICORP_TERRAFORM_TIME_VERSION}/linux_${ARCH}/terraform-provider-time_v${HASHICORP_TERRAFORM_TIME_VERSION}"
popd

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

### tailscale ###
if [ -d tailscale ]; then
    pushd tailscale
    git reset --hard && git fetch --depth 1 origin "v${TAILSCALE_VERSION}:refs/tags/v${TAILSCALE_VERSION}" && git checkout "v${TAILSCALE_VERSION}"
    popd
else
    git clone https://github.com/tailscale/tailscale.git tailscale --depth 1 -b "v${TAILSCALE_VERSION}"
fi

pushd tailscale

TAGS="$(go run ./cmd/featuretags -add cli,debug,dns,osrouter,advertiseroutes,useroutes,resolved,tailnetlock,unixsocketidentity -min)" ./build_dist.sh tailscale.com/cmd/tailscaled
strip tailscaled
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
go build -o migration-manager ./cmd/migration-manager
go build -o migration-manager-worker ./cmd/migration-manager-worker
strip migration-managerd migration-manager migration-manager-worker

# Limit building of the Migration Manager worker image to amd64, since the vmware vddk isn't available for arm64.
if [ "${ARCH}" = "amd64" ]; then
    pushd worker
    make build
    popd
fi

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
go build -o operations-center ./cmd/operations-center
strip operations-centerd operations-center

pushd ui
YARN_ENABLE_HARDENED_MODE=0 YARN_ENABLE_IMMUTABLE_INSTALLS=false yarnpkg install && yarnpkg build
popd

popd
