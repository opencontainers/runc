#!/bin/bash
# Copyright (C) 2026 Open Containers Authors
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

set -xEeuo pipefail

# Commits that bats-core release tags point to. Git verifies the checked
# out tree against the commit hash, so pinning it protects against a tag
# being moved (or anything else being served instead). To add a version:
#   git ls-remote https://github.com/bats-core/bats-core "refs/tags/v$VERSION^{}"
declare -A BATS_COMMIT=(
	["1.13.0"]=3bca150ec86275d6d9d5a4fd7d48ab8b6c6f3d87
)

# Download, verify, and install bats-core.
# Parameters:
#  $1 -- bats version to install.
#  $2 -- installation prefix.
function install_bats() {
	local ver="$1"
	local prefix="$2"
	local commit="${BATS_COMMIT[${ver}]}"

	local srcdir
	srcdir="$(mktemp -d)"
	# shellcheck disable=SC2064 # We want to resolve the path here.
	trap "rm -rf '$srcdir'" EXIT

	git clone -c advice.detachedHead=false --depth 1 --branch "v${ver}" \
		https://github.com/bats-core/bats-core.git "$srcdir"
	local head
	head="$(git -C "$srcdir" rev-parse HEAD)"
	if [ "$head" != "$commit" ]; then
		echo "bats v${ver} is commit $head, expected $commit" >&2
		exit 1
	fi

	"$srcdir"/install.sh "$prefix"
}

if [ $# -ne 2 ]; then
	echo "Usage: $0 <version> <prefix>" >&2
	exit 1
fi

install_bats "$@"
