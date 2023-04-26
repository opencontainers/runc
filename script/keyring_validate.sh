#!/bin/bash
# Copyright (C) 2023 SUSE LLC.
# Copyright (C) 2023 Open Containers Authors
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

function log() {
	echo "[*]" "$@" >&2
}

function bail() {
	log "$@"
	exit 1
}

# Temporary GPG keyring for messing around with.
tmp_gpgdir="$(mktemp -d --tmpdir "$project-validate-tmpkeyring.XXXXXX")"
trap 'rm -r "$tmp_gpgdir"' EXIT

# Get the set of MAINTAINERS.
readarray -t maintainers < <(sed -E 's|.* <.*> \(@?(.*)\)$|\1|' <"$root/MAINTAINERS")
echo "------------------------------------------------------------"
echo "$project maintainers:"
printf " * %s\n" "${maintainers[@]}"
echo "------------------------------------------------------------"

# Create a dummy gpg keyring from the set of MAINTAINERS.
while IFS="" read -r username || [ -n "$username" ]; do
	curl -sSL "https://github.com/$username.gpg" |
		gpg --no-default-keyring --keyring="$tmp_gpgdir/$username.keyring" --import
done < <(printf '%s\n' "${maintainers[@]}")

# Make sure all of the keys in the keyring have a github=... comment.
awk <"$root/$project.keyring" '
	/^-----BEGIN PGP PUBLIC KEY BLOCK-----$/ { key_idx++; in_pgp=1; has_comment=0; }

	# PGP comments are never broken up over several lines, and we only have one
	# comment entry in our keyring file anyway.
	in_pgp && /^Comment:.* github=\w+.*/ { has_comment=1 }

	/^-----END PGP PUBLIC KEY BLOCK-----$/ {
		if (!has_comment) {
			print "[!] Key", key_idx, "in '$project'.keyring is missing a github= comment."
			exit 1
		}
	}
'

echo "------------------------------------------------------------"
echo "$project release managers:"
sed -En "s|^Comment:.* github=(\w+).*| * \1|p" <"$root/$project.keyring" | sort -u
echo "------------------------------------------------------------"
gpg --no-default-keyring --keyring="$tmp_gpgdir/keyring" \
	--import --import-options=show-only <"$root/$project.keyring"
echo "------------------------------------------------------------"

# Check that each entry in the kering is actually a maintainer's key.
while IFS="" read -d $'\0' -r block || [ -n "$block" ]; do
	username="$(sed -En "s|^Comment:.* github=(\w+).*|\1|p" <<<"$block")"

	# FIXME: This is to work around codespell thinking that f-p-r is a
	# misspelling of some other word, and the lack of support for inline
	# ignores in codespell.
	fprfield="f""p""r"

	# Check the username is actually a maintainer. This is just a sanity check,
	# since you can put whatever you like in the Comment field.
	[ -f "$tmp_gpgdir/$username.keyring" ] || bail "User $username in runc.keyring is not a maintainer!"
	grep "(@$username)$" "$root/MAINTAINERS" >/dev/null || bail "User $username in runc.keyring is not a maintainer!"

	# Check that the key in the block actually matches a known key for that
	# maintainer. Note that a block can contain multiple keys, so we need to
	# check all of them. Since we have to handle multiple keys anyway, we'll
	# also verify all of the subkeys (this is simpler to implement anyway since
	# the --with-colons format outputs fingerprints for both primary and
	# subkeys in the same way).
	#
	# Fingerprints have a field 1 of $fprfield and field 10 containing the
	# fingerprint. See <https://github.com/gpg/gnupg/blob/master/doc/DETAILS>
	# for more details.
	while IFS="" read -r key || [ -n "$key" ]; do
		gpg --no-default-keyring --keyring="$tmp_gpgdir/$username.keyring" \
			--list-keys --with-colons | grep "$fprfield:::::::::$key:" >/dev/null ||
			bail "(Sub?)Key $key in $project.keyring is NOT actually one of $username's keys!"
		log "Successfully verified $username's (sub?)key $key is legitimate."
	done < <(gpg --no-default-keyring \
		--import --import-options=show-only --with-colons <<<"$block" |
		grep "^$fprfield:" | cut -d: -f10)
done < <(awk <"$root/$project.keyring" '
	/^-----BEGIN PGP PUBLIC KEY BLOCK-----$/ { in_block=1 }
	in_block { print }
	/^-----END PGP PUBLIC KEY BLOCK-----$/   { in_block=0; printf("\0"); }
')
