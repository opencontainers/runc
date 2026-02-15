#!/bin/bash
# Copyright (C) 2017 SUSE LLC.
# Copyright (C) 2017-2021 Open Containers Authors
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

set -e

## --->
# Project-specific options and functions. In *theory* you shouldn't need to
# touch anything else in this script in order to use this elsewhere.
: "${LIBSECCOMP_VERSION:=2.6.0}"
: "${LIBPATHRS_VERSION:=0.2.3}"
project="runc"
root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

# shellcheck source=./script/lib.sh
source "$root/script/lib.sh"

# This function takes an output path as an argument, where the built
# (preferably static) binary should be placed.
# Parameters:
#   $1 -- destination directory to place build artefacts to.
#   $2 -- native architecture (a .suffix for a native binary file name).
#   $@ -- additional architectures to cross-build for.
function build_project() {
	local builddir
	builddir="$(dirname "$1")"
	shift
	local native_arch="$1"
	shift
	local arches=("$@")

	# Assume that if /opt/runc-dylibs exists, then we are running via
	# Dockerfile, and thus seccomp is already built. Otherwise, build it now.
	local dylibdir=/opt/runc-dylibs
	if ! [ -d "$dylibdir" ]; then
		trap 'rm -rf "$dylibdir"' EXIT
		dylibdir="$(mktemp -d)"
		# Download and build libseccomp.
		"$root/script/build-seccomp.sh" "$LIBSECCOMP_VERSION" "$dylibdir" "${arches[@]}"
		# Download and build libpathrs.
		"$root/script/build-libpathrs.sh" "$LIBPATHRS_VERSION" "$dylibdir" "${arches[@]}"
	fi

	# For reproducible builds, add these to EXTRA_LDFLAGS:
	#  -w to disable DWARF generation;
	#  -s to disable symbol table;
	#  -buildid= to remove variable build id.
	local ldflags="-w -s -buildid="
	# Add -a to go build flags to make sure it links against
	# the provided libseccomp, not the system one (otherwise
	# it can reuse cached pkg-config results).
	local make_args=(COMMIT_NO= EXTRA_FLAGS="-a" EXTRA_LDFLAGS="${ldflags}" static)

	# Save the original cflags.
	local original_cflags="${CFLAGS:-}"

	# Build for all requested architectures.
	local arch
	for arch in "${arches[@]}"; do
		# Reset CFLAGS.
		CFLAGS="$original_cflags"
		set_cross_vars "$arch"
		make -C "$root" \
			PKG_CONFIG_PATH="$dylibdir/$arch/lib/pkgconfig" \
			"${make_args[@]}"
		"$STRIP" "$root/$project"
		mv "$root/$project" "$builddir/$project.$arch"
	done

	# Sanity check: make sure libseccomp version is as expected.
	local ver
	ver=$("$builddir/$project.$native_arch" --version | awk '$1 == "libseccomp:" {print $2}')
	if [ "$ver" != "$LIBSECCOMP_VERSION" ]; then
		echo >&2 "libseccomp version mismatch: want $LIBSECCOMP_VERSION, got $ver"
		exit 1
	fi

	# Copy libseccomp source tarball.
	cp "$dylibdir"/src/* "$builddir"
}

# End of the easy-to-configure portion.
## <---

# Print usage information.
function usage() {
	echo "usage: release_build.sh [-a <cross-arch>]... [-c <commit-ish>] [-H <hashcmd>]" >&2
	echo "                        [-r <release-dir>] [-v <version>]" >&2
	exit 1
}

# Log something to stderr.
function log() {
	echo "[*] $*" >&2
}

# Log something to stderr and then exit with 0.
function bail() {
	log "$@"
	exit 0
}

# When creating releases we need to build static binaries, an archive of the
# current commit, and generate detached signatures for both.
commit="HEAD"
version=""
releasedir=""
hashcmd=""
# Always build a native binary.
native_arch="$(go env GOARCH || echo "amd64")"
arches=("$native_arch")

while getopts "a:c:H:hr:v:" opt; do
	case "$opt" in
	a)
		# Add architecture if not already present in arches.
		if ! (printf "%s\0" "${arches[@]}" | grep -zqxF "$OPTARG"); then
			arches+=("$OPTARG")
		fi
		;;
	c)
		commit="$OPTARG"
		;;
	H)
		hashcmd="$OPTARG"
		;;
	h)
		usage
		;;
	r)
		releasedir="$OPTARG"
		;;
	v)
		version="$OPTARG"
		;;
	:)
		echo "Missing argument: -$OPTARG" >&2
		usage
		;;
	\?)
		echo "Invalid option: -$OPTARG" >&2
		usage
		;;
	esac
done

version="${version:-$(<"$root/VERSION")}"
releasedir="${releasedir:-release/$version}"
hashcmd="${hashcmd:-sha256sum}"

log "creating $project release in '$releasedir'"
log "  version: $version"
log "   commit: $commit"
log "     hash: $hashcmd"

# Make explicit what we're doing.
set -x

# Make the release directory.
rm -rf "$releasedir" && mkdir -p "$releasedir"

# Build project.
build_project "$releasedir/$project" "$native_arch" "${arches[@]}"

# Generate new archive.
git archive --format=tar --prefix="$project-$version/" "$commit" | xz >"$releasedir/$project-$version.tar.xz"

# Generate sha256 checksums for binaries and libseccomp tarball.
(
	cd "$releasedir"
	# Add hash of all architecture binaries ($project.$arch).
	"$hashcmd" "${arches[@]/#/$project.}" >>"$project.$hashcmd"
	# Add hash of tarball ($project-$version.tar.xz).
	"$hashcmd" "$project-$version.tar.xz" >>"$project.$hashcmd"
)
