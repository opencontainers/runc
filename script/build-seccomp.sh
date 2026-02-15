#!/bin/bash

set -e -u -o pipefail

# shellcheck source=./script/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# sha256 checksums for seccomp release tarballs.
declare -A SECCOMP_SHA256=(
	["2.5.5"]=248a2c8a4d9b9858aa6baf52712c34afefcf9c9e94b76dce02c1c9aa25fb3375
	["2.5.6"]=04c37d72965dce218a0c94519b056e1775cf786b5260ee2b7992956c4ee38633
	["2.6.0"]=83b6085232d1588c379dc9b9cae47bb37407cf262e6e74993c61ba72d2a784dc
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

	# Install native version for Dockerfile builds.
	./configure \
		--prefix="$dest" --libdir="$dest/lib" \
		--enable-static --enable-shared
	make install
	make clean

	# Save the original cflags.
	local original_cflags="${CFLAGS:-}"

	# Build and install for all requested architectures.
	local arch
	for arch in "${arches[@]}"; do
		# Reset CFLAGS.
		CFLAGS="$original_cflags"
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
	echo "Usage: $0 <version> <dest-dir> [<extra-arch> ...]" >&2
	exit 1
fi

build_libseccomp "$@"
