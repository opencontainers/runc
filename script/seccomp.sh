#!/bin/bash

# shellcheck source=./script/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# Due to libseccomp being LGPL we must include its sources,
# so download, install and build against it.
# Parameters:
#  $1 -- libseccomp version to download and build.
#  $2 -- destination directory to put the source tarball in.
#  $3 -- file to append LIBSECCOMP_PREFIX*= environment variables to
#        (can be sourced to get install paths).
#  $@ -- additional architectures to cross-compile for.
function build_libseccomp() {
	local ver="$1"
	shift
	local dest="$1"
	shift
	local varfile="$1"
	shift
	local arches=("$@")
	local tar="libseccomp-${ver}.tar.gz"

	# Download and extract.
	wget "https://github.com/seccomp/libseccomp/releases/download/v${ver}/${tar}"{,.asc}
	local srcdir
	srcdir="$(mktemp -d)"
	tar xf "$tar" -C "$srcdir"
	pushd "$srcdir/libseccomp-$ver" || return

	# Build and install natively.
	local prefix
	prefix="$(mktemp -d)"
	./configure \
		--prefix="$prefix" --libdir="$prefix/lib" \
		--enable-static --disable-shared
	echo LIBSECCOMP_PREFIX="$prefix" >>"$varfile"
	make install
	make clean

	# Build and install for additional architectures.
	local arch
	for arch in "${arches[@]}"; do
		prefix="$(mktemp -d)"
		set_cross_vars "$arch"
		./configure --host "$HOST" \
			--prefix="$prefix" --libdir="$prefix/lib" \
			--enable-static --enable-shared
		make install
		make clean
		echo "LIBSECCOMP_PREFIX_${arch}=$prefix" >>"$varfile"
	done

	# Place the source tarball to $dest.
	popd || return
	mv "$tar"{,.asc} "$dest"
}

if $# -lt 4; then
	echo "Usage: seccomp.sh <version> <dest-dir> <var-file> [<extra-arch> ...]" >&2
	exit 1
fi

build_libseccomp "$@"
