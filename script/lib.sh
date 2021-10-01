#!/bin/bash

# set_cross_vars sets a few environment variables used for cross-compiling,
# based on the architecture specified in $1.
function set_cross_vars() {
	GOARCH="$1" # default, may be overridden below
	unset GOARM

	case $1 in
	arm64)
		HOST=aarch64-linux-gnu
		;;
	armel)
		HOST=arm-linux-gnueabi
		GOARCH=arm
		GOARM=6
		;;
	armhf)
		HOST=arm-linux-gnueabihf
		GOARCH=arm
		GOARM=7
		;;
	ppc64le)
		HOST=powerpc64le-linux-gnu
		;;
	s390x)
		HOST=s390x-linux-gnu
		;;
	*)
		echo "set_cross_vars: unsupported architecture: $1" >&2
		exit 1
		;;
	esac

	CC=$HOST-gcc
	STRIP=$HOST-strip

	export HOST GOARM GOARCH CC STRIP
}
