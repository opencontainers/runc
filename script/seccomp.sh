#!/bin/bash

set -e -u -o pipefail

# shellcheck source=./script/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# sha256 checksums for seccomp release tarballs.
declare -A SECCOMP_SHA256=(
	["2.5.5"]=248a2c8a4d9b9858aa6baf52712c34afefcf9c9e94b76dce02c1c9aa25fb3375
)

# Due to libseccomp being LGPL we must include its sources,
# so download, install and build against it.
# Parameters:
#  $1 -- libseccomp version to download and build.
#  $2 -- destination directory.
#  $@ -- additional architectures to cross-compile for.
function build_libseccomp() {
	local ver="$1"
	shift
	local dest="$1"
	shift
	local arches=("$@")
	local tar="libseccomp-${ver}.tar.gz"

	# Download, check, and extract.
	wget "https://github.com/seccomp/libseccomp/releases/download/v${ver}/${tar}"{,.asc}
	sha256sum --strict --check - <<<"${SECCOMP_SHA256[${ver}]} *${tar}"

	local srcdir
	srcdir="$(mktemp -d)"
	tar xf "$tar" -C "$srcdir"
	pushd "$srcdir/libseccomp-$ver" || return

	# Build natively and install to /usr/local.
	./configure \
		--prefix="$dest" --libdir="$dest/lib" \
		--enable-static --enable-shared
	make install
	make clean

	# Build and install for additional architectures.
	local arch
	for arch in "${arches[@]}"; do
		set_cross_vars "$arch"
		./configure --host "$HOST" \
			--prefix="$dest/$arch" --libdir="$dest/$arch/lib" \
			--enable-static --enable-shared
		make install
		make clean
	done

	# Place the source tarball to $dest/src.
	popd || return
	mkdir "$dest"/src
	mv "$tar"{,.asc} "$dest"/src
}

if [ $# -lt 2 ]; then
	echo "Usage: seccomp.sh <version> <dest-dir> [<extra-arch> ...]" >&2
	exit 1
fi

build_libseccomp "$@"
