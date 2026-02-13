#!/bin/bash

# set_cross_vars sets a few environment variables used for cross-compiling,
# based on the architecture specified in $1.
function set_cross_vars() {
	GOARCH="$1" # default, may be overridden below
	unset GOARM

	PLATFORM=linux-gnu
	# openSUSE has a custom PLATFORM
	if grep -iq "ID_LIKE=.*suse" /etc/os-release; then
		PLATFORM=suse-linux
		is_suse=1
	fi

	case $1 in
	386)
		# Always use the 64-bit compiler to build the 386 binary, which works
		# for the more common cross-build method for x86 (namely, the
		# equivalent of dpkg --add-architecture).
		local cpu_type
		if [ -v is_suse ]; then
			# There is no x86_64-suse-linux-gcc, so use the native one.
			HOST=
			cpu_type=i586
		else
			HOST=x86_64-${PLATFORM}
			cpu_type=i686
		fi
		CFLAGS="-m32 -march=$cpu_type ${CFLAGS[*]}"
		;;
	amd64)
		if [ -n "${is_suse:-}" ]; then
			# There is no x86_64-suse-linux-gcc, so use the native one.
			HOST=
		else
			HOST=x86_64-${PLATFORM}
		fi
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

	export HOST CFLAGS GOARM GOARCH CC STRIP
}
