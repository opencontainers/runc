#!/bin/bash
# Copyright (C) 2017 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# rootless.sh -- Runner for rootless container tests. The purpose of this
# script is to allow for the addition (and testing) of "opportunistic" features
# to rootless containers while still testing the base features. In order to add
# a new feature, please match the existing style. Add an entry to $ALL_FEATURES,
# and add an enable_* and disable_* hook.

ALL_FEATURES=()
ROOT="$(readlink -f "$(dirname "${BASH_SOURCE}")/..")"

# Create a powerset of $ALL_FEATURES (the set of all subsets of $ALL_FEATURES).
# We test all of the possible combinations (as long as we don't add too many
# feature knobs this shouldn't take too long -- but the number of tested
# combinations is O(2^n)).
function powerset() {
	eval printf '%s' $(printf '{,%s+}' "$@"):
}
features_powerset="$(powerset "${ALL_FEATURES[@]}")"

# Iterate over the powerset of all features.
IFS=:
for enabled_features in $features_powerset
do
	idx="$(($idx+1))"
	echo "[$(printf '%.2d' "$idx")] run rootless tests ... (${enabled_features%%+})"

	unset IFS
	for feature in "${ALL_FEATURES[@]}"
	do
		hook_func="disable_$feature"
		grep -E "(^|\+)$feature(\+|$)" <<<$enabled_features &>/dev/null && hook_func="enable_$feature"
		"$hook_func"
	done

	# Run the test suite!
	set -e
	echo path: $PATH
	export ROOTLESS_FEATURES="$enabled_features"
	sudo -HE -u rootless PATH="$PATH" bats -t "$ROOT/tests/integration"
	set +e
done
