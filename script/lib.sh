#!/bin/bash

# NOTE: Make sure you keep this file in sync with cc_platform.mk.

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
		# "armhf" means ARMv7 for Debian, ARMv6 for Raspbian.
		# ARMv6 is chosen here for compatibility.
		#
		# https://wiki.debian.org/RaspberryPi
		#
		# > Raspberry Pi OS builds a single image for all of the Raspberry families,
		# > so you will get an armhf 32-bit, hard floating-point system, but built
		# > for the ARMv6 ISA (with VFP2), unlike Debian's ARMv7 ISA (with VFP3)
		# > port.
		GOARM=6
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
