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

project="runc"
root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

# Print usage information.
function usage() {
	echo "usage: release_sign.sh [-S <gpg-key-id>] [-H <hashcmd>]" >&2
	echo "                       [-r <release-dir>] [-v <version>]" >&2
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

# Conduct a sanity-check to make sure that GPG provided with the given
# arguments can sign something. Inability to sign things is not a fatal error.
function gpg_cansign() {
	gpg "$@" --clear-sign </dev/null >/dev/null
}

# When creating releases we need to build static binaries, an archive of the
# current commit, and generate detached signatures for both.
keyid=""
version=""
releasedir=""
hashcmd=""

while getopts "H:hr:S:v:" opt; do
	case "$opt" in
	H)
		hashcmd="$OPTARG"
		;;
	h)
		usage
		;;
	r)
		releasedir="$OPTARG"
		;;
	S)
		keyid="$OPTARG"
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

log "signing $project release in '$releasedir'"
log "      key: ${keyid:-DEFAULT}"
log "     hash: $hashcmd"

# Make explicit what we're doing.
set -x

# Set up the gpgflags.
gpgflags=()
[[ "$keyid" ]] && gpgflags=(--default-key "$keyid")
gpg_cansign "${gpgflags[@]}" || bail "Could not find suitable GPG key, skipping signing step."

# Only needed for local signing -- change the owner since by default it's built
# inside a container which means it'll have the wrong owner and permissions.
[ -w "$releasedir" ] || sudo chown -R "$USER:$GROUP" "$releasedir"

# Sign everything.
for bin in "$releasedir/$project".*; do
	[[ "$(basename "$bin")" == "$project.$hashcmd" ]] && continue # skip hash
	gpg "${gpgflags[@]}" --detach-sign --armor "$bin"
done
gpg "${gpgflags[@]}" --clear-sign --armor \
	--output "$releasedir/$project.$hashcmd"{.tmp,} &&
	mv "$releasedir/$project.$hashcmd"{.tmp,}
