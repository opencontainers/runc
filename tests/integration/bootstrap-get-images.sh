#!/usr/bin/env bash
set -Eeuo pipefail

# This script generates "get-images.sh" using Official Images tooling.
#
#   ./bootstrap-get-images.sh > get-images.sh
#
# This script requires "bashbrew". To get the latest version, visit
# https://github.com/docker-library/bashbrew/releases

images=(
	# pinned to an older BusyBox (prior to 1.36 becoming "latest") because 1.36.0 has some unresolved bugs, especially around sha256sum
	'https://github.com/docker-library/official-images/raw/eaed422a86b43c885a0f980d48f4bbf346086a4a/library/busybox:glibc'

	# pinned to an older Debian Buster which has more architectures than the latest does (Buster transitioned from the Debian Security Team to the LTS Team which supports a smaller set)
	'https://github.com/docker-library/official-images/raw/ce10f6b60289c0c0b5de6f785528b8725f225a58/library/debian:buster-slim'
)

cat <<'EOH'
#!/bin/bash

# DO NOT EDIT!  Generated by "bootstrap-get-images.sh".

# This script checks if container images needed for tests (currently
# busybox and Debian 10 "Buster") are available locally, and downloads
# them to testdata directory if not.
#
# The script is self-contained/standalone and is used from a few places
# that need to ensure the images are downloaded. Its output is suitable
# for consumption by shell via eval (see helpers.bash).

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
if [ "$arch" = 'arm' ]; then
	arm=$(go env GOARM)
	: "${arm:=7}"
	arch=${arch}v$arm
fi
EOH

# shellcheck disable=SC2016 # this generates shell code intentionally (and many of the '$' in here are intended for "text/template" not the end shell anyhow)
bashbrew cat --format '
	{{- "\n" -}}
	{{- "case $arch in\n" -}}

	{{- range .TagEntry.Architectures -}}
		{{- $repo := $.TagEntry.ArchGitRepo . | trimSuffixes ".git" -}}
		{{- $branch := $.TagEntry.ArchGitFetch . | trimPrefixes "refs/heads/" -}}
		{{- $commit := $.TagEntry.ArchGitCommit . -}}
		{{- $dir := $.TagEntry.ArchDirectory . -}}
		{{- $tarball := eq $.RepoName "debian" | ternary "rootfs.tar.xz" "busybox.tar.xz" -}}

		{{ . | replace "arm64v8" "arm64" "arm32" "arm" "i386" "386" }} {{- ")\n" -}}
		{{- "\t" -}}# {{ $repo }}/tree/{{ $branch }}{{- "\n" -}}
		{{- "\t" -}}# {{ $repo }}/tree/{{ $commit }}/{{ $dir }}{{- "\n" -}}
		{{- "\t" -}} url="{{ $repo }}/raw/{{ $commit }}/{{ $dir }}/{{ $tarball }}"{{- "\n" -}}
		{{- "\t" -}} ;; {{- "\n" -}}
		{{- "\n" -}}
	{{- end -}}

	*){{- "\n" -}}
	{{- "\t" -}}echo >&2 "error: unsupported {{ $.RepoName }} architecture: $arch"{{- "\n" -}}
	{{- "\t" -}}exit 1{{- "\n" -}}
	{{- "\t" -}};;{{- "\n" -}}

	{{- "esac\n" -}}
	{{- printf `rootfs="$TESTDATA/%s-${arch}.tar.xz"` $.RepoName -}}{{- "\n" -}}
	{{- `get "$rootfs" "$url"` -}}{{- "\n" -}}
	{{- printf "var=%s_image\n" $.RepoName -}}
	{{- `echo "${var^^}=$rootfs"` -}}
' "${images[@]}"
