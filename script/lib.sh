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

# set_cross_vars sets a few environment variables used for cross-compiling,
# based on the architecture specified in $1.
function set_cross_vars() {
	GOARCH="$1" # default, may be overridden below
	unset GOARM

	PLATFORM="$(get_platform)"

	case "$1" in
	amd64)
		HOST=x86_64-${PLATFORM}
		;;
	arm64)
		HOST=aarch64-${PLATFORM}
		;;
	armel)
		HOST=arm-${PLATFORM}eabi
		GOARCH=arm
		GOARM=5
		;;
	armhf)
		HOST=arm-${PLATFORM}eabihf
		GOARCH=arm
		GOARM=7
		;;
	ppc64le)
		HOST=powerpc64le-${PLATFORM}
		;;
	riscv64)
		HOST=riscv64-${PLATFORM}
		;;
	s390x)
		HOST=s390x-${PLATFORM}
		;;
	*)
		echo "set_cross_vars: unsupported architecture: $1" >&2
		exit 1
		;;
	esac

	CC="${HOST:+$HOST-}gcc"
	STRIP="${HOST:+$HOST-}strip"

	export HOST GOARM GOARCH CC STRIP
}
