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

set -Eeuxo pipefail

root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"
keyring_file="$root/runc.keyring"

function bail() {
	echo "$@" >&2
	exit 1
}

[[ "$#" -eq 2 ]] || bail "usage: $0 <github-handle> <keyid>"

github_handle="${1}"
gpg_keyid="${2}"

cat >>"$keyring_file" <<EOF
$(gpg --list-keys "$gpg_keyid")

$(gpg --armor --comment="github=$github_handle" --export --export-options=export-minimal,export-clean "$gpg_keyid")

EOF
