#!/bin/bash

set -e -u -o pipefail

# shellcheck source=./script/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

function build_libtpms() {
    local libtpms_ver="$1"
    wget "https://github.com/stefanberger/libtpms/archive/refs/tags/v${libtpms_ver}.tar.gz"

    local tpms_srcdir
       tpms_srcdir="$(mktemp -d)"
    tar xf "v${libtpms_ver}.tar.gz" -C "$tpms_srcdir"
    echo $(ls -la $tpms_srcdir)
    pushd "$tpms_srcdir/libtpms-${libtpms_ver}" || return
    ./autogen.sh --with-openssl --enable-debug
    make dist
    mv debian/source debian/source.old
    dpkg-buildpackage -us -uc -j4
    pushd "$tpms_srcdir" || return
    dpkg -i "libtpms0_${libtpms_ver}_amd64.deb" "libtpms-dev_${libtpms_ver}_amd64.deb"
    popd || return
    popd || return
}

function build_swtpm() {
    local swtpm_ver="$1"
    local libtpms_ver="$2"

    build_libtpms $libtpms_ver

    wget "https://github.com/stefanberger/swtpm/archive/refs/tags/v${swtpm_ver}.tar.gz"

    local swtpm_srcdir
    swtpm_srcdir="$(mktemp -d)"
    tar xf "v${swtpm_ver}.tar.gz" -C "$swtpm_srcdir"
    echo $(ls -la $swtpm_srcdir)
    pushd "$swtpm_srcdir/swtpm-${swtpm_ver}" || return
    ./autogen.sh --with-openssl --with-cuse --prefix=/usr --enable-debug
    make -j4
    # make -j4 check
    make install
    popd || return
}

if [ $# -lt 2 ]; then
       echo "Usage: swtpm.sh <swtpm_version> <libtpms_version>" >&2
       exit 1
fi

build_swtpm "$@"
