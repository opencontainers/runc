#!/bin/bash

# get_platform computes the platform section of target triples on this OS.
function get_platform() {
	# Fedora doesn't have ID_LIKE and only has ID=fedora, so we need to
	# construct a fake ID_LIKE to treat AlmaLinux and Fedora the same way.
	local ID_LIKE
	# shellcheck source=/etc/os-release disable=SC1091 # outside our sources
	ID_LIKE="$(
		source /etc/os-release
		echo "${ID:-} ${ID_LIKE:-}"
	)"

	local PLATFORM
	case "$ID_LIKE" in
	*suse*)
		PLATFORM=suse-linux
		;;
	*rhel* | *fedora* | *centos*)
		PLATFORM=redhat-linux
		;;
	*)
		PLATFORM=linux-gnu
		;;
	esac
	echo "$PLATFORM"
}

# set_cross_vars sets a few environment variables used for cross-compiling
# (against musl libc, except for s390x), based on the architecture specified
# in $1.
#
# It relies on Debian's gcc cross-compilers and musl-dev packages (for each
# target architecture), and Linux kernel headers available to musl (see
# Dockerfile).
function set_cross_vars() {
	GOARCH="$1" # default, may be overridden below
	local musl
	unset GOARM

	PLATFORM="$(get_platform)"

	case "$1" in
	amd64)
		HOST=x86_64-${PLATFORM}
		musl=x86_64-linux-musl
		;;
	arm64)
		HOST=aarch64-${PLATFORM}
		musl=aarch64-linux-musl
		;;
	armel)
		HOST=arm-${PLATFORM}eabi
		musl=arm-linux-musleabi
		GOARCH=arm
		GOARM=5
		;;
	armhf)
		HOST=arm-${PLATFORM}eabihf
		musl=arm-linux-musleabihf
		GOARCH=arm
		GOARM=7
		;;
	ppc64le)
		HOST=powerpc64le-${PLATFORM}
		musl=powerpc64le-linux-musl
		;;
	riscv64)
		HOST=riscv64-${PLATFORM}
		musl=riscv64-linux-musl
		;;
	s390x)
		# Use glibc, since for s390x-unknown-linux-musl (a Tier 3 Rust
		# target) there is no prebuilt Rust standard library (needed to
		# build libpathrs) nor unwinder, and libpathrs does not compile.
		#
		# TODO: switch to musl once a libpathrs release with the fix
		# (https://github.com/cyphar/libpathrs/pull/426) is available,
		# rebuilding the Rust standard library (-Zbuild-std) and building
		# LLVM libunwind from the rust-src component.
		HOST=s390x-${PLATFORM}
		;;
	*)
		echo "set_cross_vars: unsupported architecture: $1" >&2
		exit 1
		;;
	esac

	CC="${HOST}-gcc${musl:+ -specs=/usr/lib/${musl}/musl-gcc.specs}"
	STRIP="${HOST}-strip"

	export HOST GOARM GOARCH CC STRIP
}
