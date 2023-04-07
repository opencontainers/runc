#!/bin/bash
# Copyright (C) 2017-2023 SUSE LLC.
# Copyright (C) 2017-2023 Open Containers Authors
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

set -Eeuo pipefail

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
	echo "[*]" "$@" >&2
}

# Log something to stderr and then exit with 0.
function quit() {
	log "$@"
	exit 0
}

# Log something to stderr and then exit with 1.
function bail() {
	log "$@"
	exit 1
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

# Set up the gpgflags.
gpgflags=()
[[ "$keyid" ]] && gpgflags=(--default-key "$keyid")
gpg_cansign "${gpgflags[@]}" || quit "Could not find suitable GPG key, skipping signing step."

# Make explicit what we're doing.
set -x

# Check that the keyid is actually in the $project.keyring by signing a piece
# of dummy text then verifying it against the list of keys in that keyring.
tmp_gpgdir="$(mktemp -d --tmpdir "$project-sign-tmpkeyring.XXXXXX")"
trap 'rm -r "$tmp_gpgdir"' EXIT

tmp_runc_gpgflags=("--no-default-keyring" "--keyring=$tmp_gpgdir/$project.keyring")
gpg "${tmp_runc_gpgflags[@]}" --import <"$root/$project.keyring"

tmp_seccomp_gpgflags=("--no-default-keyring" "--keyring=$tmp_gpgdir/seccomp.keyring")
gpg "${tmp_seccomp_gpgflags[@]}" --recv-keys 0x47A68FCE37C7D7024FD65E11356CE62C2B524099
gpg "${tmp_seccomp_gpgflags[@]}" --recv-keys 0x7100AADFAE6E6E940D2E0AD655E45A5AE8CA7C8A

gpg "${gpgflags[@]}" --clear-sign <<<"[This is test text used for $project release scripts. $(date --rfc-email)]" |
	gpg "${tmp_runc_gpgflags[@]}" --verify || bail "Signing key ${keyid:-DEFAULT} is not in trusted $project.keyring list!"

# Make sure the signer is okay with the list of keys in the keyring (once this
# release is signed, distributions will trust this keyring).
cat >&2 <<EOF
== PLEASE VERIFY THE FOLLOWING KEYS ==

The sources for this release will contain the following signing keys as
"trusted", meaning that distributions may trust the keys to sign future
releases. Please make sure that only authorised users' keys are listed.

$(gpg "${tmp_runc_gpgflags[@]}" --list-keys)

[ Press ENTER to continue. ]
EOF
read -r

# Only needed for local signing -- change the owner since by default it's built
# inside a container which means it'll have the wrong owner and permissions.
[ -w "$releasedir" ] || sudo chown -R "$(id -u):$(id -g)" "$releasedir"

# Sign everything.
for bin in "$releasedir/$project".*; do
	[[ "$(basename "$bin")" == "$project.$hashcmd" ]] && continue # skip hash
	gpg "${gpgflags[@]}" --detach-sign --armor "$bin"
done
gpg "${gpgflags[@]}" --clear-sign --armor \
	--output "$releasedir/$project.$hashcmd"{.tmp,} &&
	mv "$releasedir/$project.$hashcmd"{.tmp,}

# Verify that all the signatures and shasum are correct.
pushd "$releasedir"

# Verify project-signed detached signatures.
find . -name "$project.*.asc" -print0 | xargs -0 -L1 gpg "${tmp_runc_gpgflags[@]}" --verify --

# Verify shasum.
"$hashcmd" -c "$project.$hashcmd"
gpg "${tmp_runc_gpgflags[@]}" --verify "$project.$hashcmd"

# Verify seccomp tarball.
gpg "${tmp_seccomp_gpgflags[@]}" --verify libseccomp*.asc

popd
