#!/bin/bash

set -e

### Versions ###
KPX_VERSION=1.12.1

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
