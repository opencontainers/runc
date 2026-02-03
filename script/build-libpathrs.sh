#!/bin/bash
# Copyright (C) 2026 Open Containers Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xEeuo pipefail

# shellcheck source=./script/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

PLATFORM="$(get_platform)"

declare -A GOARCH_TO_RUST_TARGET=(
	["386"]=i686-unknown-linux-gnu
	["amd64"]=x86_64-unknown-linux-gnu
	["arm64"]=aarch64-unknown-linux-gnu
	["armel"]=armv5te-unknown-linux-gnueabi
	["armhf"]=armv7-unknown-linux-gnueabihf
	["ppc64le"]=powerpc64le-unknown-linux-gnu
	["s390x"]=s390x-unknown-linux-gnu
	["riscv64"]=riscv64gc-unknown-linux-gnu
)

declare -A RUST_TARGET_TO_CC=(
	["i686-unknown-linux-gnu"]="i686-${PLATFORM}-gcc"
	["x86_64-unknown-linux-gnu"]="x86_64-${PLATFORM}-gcc"
	["aarch64-unknown-linux-gnu"]="aarch64-${PLATFORM}-gcc"
	["armv5te-unknown-linux-gnueabi"]="arm-${PLATFORM}eabi-gcc"
	["armv7-unknown-linux-gnueabihf"]="arm-${PLATFORM}eabihf-gcc"
	["powerpc64le-unknown-linux-gnu"]="powerpc64le-${PLATFORM}-gcc"
	["s390x-unknown-linux-gnu"]="s390x-${PLATFORM}-gcc"
	["riscv64gc-unknown-linux-gnu"]="riscv64-${PLATFORM}-gcc"
)

# sha256 checksums for libpathrs release tarballs.
declare -A LIBPATHRS_SHA256=(
	["0.2.2"]=95978036c0f0d2e67f628fc06ccac090656606bee6632e437eac21d68b00504f
	["0.2.3"]=1e7826b64e41940e8f62ca92dde7ae1459a35727a3aeb172cc11d48b4727aeed
)

function generate_cargo_config() {
	for rust_target in "${GOARCH_TO_RUST_TARGET[@]}"; do
		local target_gcc="${RUST_TARGET_TO_CC[$rust_target]}"

		# Based on <https://wiki.debian.org/Rust#Crosscompiling>.
		cat <<-EOF
			[target.$rust_target]
			linker = "$target_gcc"
			rustflags = ["-L", "$(rustc --print sysroot)/lib/rustlib/$rust_target/lib"]
		EOF
	done
}

# Due to libpathrs being MPLv2/LGPLv3 we must include its sources, so
# download, install and build against it.
# Parameters:
#  $1 -- libpathrs version to download and build.
#  $2 -- destination directory.
#  $@ -- additional architectures to cross-compile for.
function build_libpathrs() {
	local ver="$1"
	shift
	local dest="$1"
	shift
	local go_arches=("$@")
	local tar="libpathrs-${ver}.tar.gz"

	# Download, check, and extract.
	# TODO: Signatures and releases.
	#wget "https://github.com/cyphar/libpathrs/releases/download/v${ver}/${tar}"{,.asc}
	#wget "https://github.com/cyphar/libpathrs/archive/refs/tags/v${ver}.tar.gz" -O "$tar"
	#sha256sum --strict --check - <<<"${LIBPATHRS_SHA256[${ver}]} *${tar}"

	local srcdir
	srcdir="$(mktemp -d)"
	#tar xf "$tar" -C "$srcdir"
	#pushd "$srcdir/libpathrs-$ver" || return
	git clone https://github.com/cyphar/libpathrs.git "$srcdir/libpathrs"
	pushd "$srcdir/libpathrs" || return

	# Use cargo-auditable if available.
	if cargo auditable --version &>/dev/null; then
		export CARGO="cargo auditable"
	fi
	extra_cargo_flags+=("--locked")

	# If we are being asked to install this in a system library directory
	# (i.e., --prefix=/usr or something similar), the correct place to put
	# libpathrs.so depends very strongly on the distro we are running on, and
	# detecting this in a generic way is quite difficult.
	#
	# The simplest solution is to use a disto-packaged binary to detect where
	# libc.so is installed and use the same import path, so we look at
	# /proc/self/maps and parse out the parent directory of the libc.so being
	# used.
	local native_libdir libdir=
	native_libdir="$(awk '$NF ~ /\/libc\>.*\.so/ { print $NF; }' /proc/self/maps | \
					 sort -u | head -n1 | xargs dirname)"
	if [[ "$native_libdir" == "$dest/"* ]]; then
		libdir="$native_libdir"
	fi

	# Install native version for Dockerfile builds.
	make \
		EXTRA_CARGO_FLAGS="${extra_cargo_flags[*]}" \
		release
	./install.sh \
		--prefix="$dest" \
		--libdir="$libdir"
	cargo clean

	local cargo_config
	cargo_config="$(mktemp --tmpdir runc-libpathrs-cargo.toml.XXXXXX)"
	# shellcheck disable=SC2064 # We want to resolve the path here.
	trap "rm -f '$cargo_config'" EXIT

	# Only configure cross-compile config when we need to cross-compile.
	# RedHat-based distros insist on calling their targets "$ARCH-redhat-linux"
	# which breaks our above logic, but we don't ever need to compile on RedHat
	# distros so we can ignore this.
	generate_cargo_config >"$cargo_config"
	extra_cargo_flags+=("--config=$cargo_config")

	for go_arch in "${go_arches[@]}"; do
		local rust_target="${GOARCH_TO_RUST_TARGET[$go_arch]}"
		make \
			EXTRA_CARGO_FLAGS="${extra_cargo_flags[*]} --target=$rust_target" \
			release
		./install.sh \
			--rust-target="$rust_target" \
			--prefix="$dest/$go_arch"
		cargo clean
	done

	popd
}

if [ $# -lt 2 ]; then
	echo "Usage: $0 <version> <dest-dir> [<extra-arch> ...]" >&2
	exit 1
fi

build_libpathrs "$@"
