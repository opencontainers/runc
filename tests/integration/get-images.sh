#!/bin/bash

# This script checks if container images needed for tests (currently
# busybox and Debian 10 "Buster") are available locally, and downloads
# them to testdata directory if not.
#
# The script is self-contained/standalone and is used from a few places
# that need to ensure the images are downloaded. Its output is suitable
# for consumption by shell via eval (see helpers.bash).
#
# XXX: Latest available images are fetched. Theoretically,
# this can bring some instability in case of a broken image.
# In this case, images will need to be pinned to a checksum
# on a per-image and per-architecture basis.

set -e -u -o pipefail

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")
# Test data path.
TESTDATA="${INTEGRATION_ROOT}/testdata"
# Sanity check: $TESTDATA directory must exist.
if [ ! -d "$TESTDATA" ]; then
	echo "Bad TESTDATA directory: $TESTDATA. Aborting" >&2
	exit 1
fi

function get() {
	local dest="$1" url="$2"

	[ -e "$dest" ] && return

	# Sanity check: $TESTDATA directory must be writable.
	if [ ! -w "$TESTDATA" ]; then
		echo "TESTDATA directory ($TESTDATA) not writable. Aborting" >&2
		exit 1
	fi

	if ! curl -o "$dest" -fsSL --retry 5 "$url"; then
		echo "Failed to get $url" 1>&2
		exit 1
	fi
}

arch=$(go env GOARCH)
# Convert from GOARCH to whatever the URLs below are using.
case $arch in
arm64)
	arch=arm64v8
	;;
386)
	arch=i386
	;;
esac

# busybox
BUSYBOX_IMAGE="$TESTDATA/busybox-${arch}.tar.xz"
get "$BUSYBOX_IMAGE" \
	"https://github.com/docker-library/busybox/raw/dist-${arch}/stable/glibc/busybox.tar.xz"
echo "BUSYBOX_IMAGE=$BUSYBOX_IMAGE"

# debian
DEBIAN_IMAGE="$TESTDATA/debian-${arch}.tar.xz"
get "$DEBIAN_IMAGE" \
	"https://github.com/debuerreotype/docker-debian-artifacts/raw/dist-${arch}/buster/slim/rootfs.tar.xz"
echo "DEBIAN_IMAGE=$DEBIAN_IMAGE"

# hello-world is local, no need to download.
HELLO_IMAGE="$TESTDATA/hello-world-${arch}.tar"
echo "HELLO_IMAGE=$HELLO_IMAGE"
