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

set -e -u -o pipefail
: "${ROOTLESS_TESTPATH:=}"

ALL_FEATURES=("idmap" "cgroup")
# cgroup is managed by systemd when RUNC_USE_SYSTEMD is set.
if [ -v RUNC_USE_SYSTEMD ]; then
	ALL_FEATURES=("idmap")
fi
ROOT="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

# FEATURE: Opportunistic new{uid,gid}map support, allowing a rootless container
#          to be set up with the usage of helper setuid binaries.

function enable_idmap() {
	export ROOTLESS_UIDMAP_START=100000 ROOTLESS_UIDMAP_LENGTH=65536
	export ROOTLESS_GIDMAP_START=200000 ROOTLESS_GIDMAP_LENGTH=65536

	# Set up sub{uid,gid} mappings.
	[ -e /etc/subuid.tmp ] && mv /etc/subuid{.tmp,}
	(
		grep -v '^rootless' /etc/subuid
		echo "rootless:$ROOTLESS_UIDMAP_START:$ROOTLESS_UIDMAP_LENGTH"
	) >/etc/subuid.tmp
	mv /etc/subuid{.tmp,}
	[ -e /etc/subgid.tmp ] && mv /etc/subgid{.tmp,}
	(
		grep -v '^rootless' /etc/subgid
		echo "rootless:$ROOTLESS_GIDMAP_START:$ROOTLESS_GIDMAP_LENGTH"
	) >/etc/subgid.tmp
	mv /etc/subgid{.tmp,}

	# Reactivate new{uid,gid}map helpers if applicable.
	[ -e /usr/bin/unused-newuidmap ] && mv /usr/bin/{unused-,}newuidmap
	[ -e /usr/bin/unused-newgidmap ] && mv /usr/bin/{unused-,}newgidmap

	# Create a directory owned by $AUX_UID inside container, to be used
	# by a test case in cwd.bats. This setup can't be done by the test itself,
	# as it needs root for chown.
	export AUX_UID=1024
	AUX_DIR="$(mktemp -d)"
	# 1000 is linux.uidMappings.containerID value,
	# as set by runc_rootless_idmap
	chown "$((ROOTLESS_UIDMAP_START - 1000 + AUX_UID))" "$AUX_DIR"
	export AUX_DIR
}

function disable_idmap() {
	export ROOTLESS_UIDMAP_START ROOTLESS_UIDMAP_LENGTH
	export ROOTLESS_GIDMAP_START ROOTLESS_GIDMAP_LENGTH

	# Deactivate sub{uid,gid} mappings.
	[ -e /etc/subuid ] && mv /etc/subuid{,.tmp}
	[ -e /etc/subgid ] && mv /etc/subgid{,.tmp}

	# Deactivate new{uid,gid}map helpers. setuid is preserved with mv(1).
	[ -e /usr/bin/newuidmap ] && mv /usr/bin/{,unused-}newuidmap
	[ -e /usr/bin/newgidmap ] && mv /usr/bin/{,unused-}newgidmap

	return 0
}

function cleanup() {
	if [ -v AUX_DIR ]; then
		rmdir "$AUX_DIR"
		unset AUX_DIX
	fi
}

# FEATURE: Opportunistic cgroups support, allowing a rootless container to set
#          resource limits on condition that cgroupsPath is set to a path the
#          rootless user has permissions on.

# List of cgroups. We handle name= cgroups as well as combined
# (comma-separated) cgroups and correctly split and/or strip them.
# shellcheck disable=SC2207
ALL_CGROUPS=($(cut -d: -f2 </proc/self/cgroup | sed -E '{s/^name=//;s/,/\n/;/^$/D}'))
CGROUP_MOUNT="/sys/fs/cgroup"
CGROUP_PATH="/runc-cgroups-integration-test"

function enable_cgroup() {
	# Set up cgroups for use in rootless containers.
	for cg in "${ALL_CGROUPS[@]}"; do
		mkdir -p "$CGROUP_MOUNT/$cg$CGROUP_PATH"
		# We only need to allow write access to {cgroup.procs,tasks} and the
		# directory. Rather than changing the owner entirely, we just change
		# the group and then allow write access to the group (in order to
		# further limit the possible DAC permissions that runc could use).
		chown root:rootless "$CGROUP_MOUNT/$cg$CGROUP_PATH/"{,cgroup.procs,tasks}
		chmod g+rwx "$CGROUP_MOUNT/$cg$CGROUP_PATH/"{,cgroup.procs,tasks}
		# Due to cpuset's semantics we need to give extra permissions to allow
		# for runc to set up the hierarchy. XXX: This really shouldn't be
		# necessary, and might actually be a bug in our impl of cgroup
		# handling.
		[ "$cg" = "cpuset" ] && chown rootless:rootless "$CGROUP_MOUNT/$cg$CGROUP_PATH/cpuset."{cpus,mems}
		# The following is required by "update rt period and runtime".
		if [ "$cg" = "cpu" ]; then
			if [[ -e "$CGROUP_MOUNT/$cg$CGROUP_PATH/cpu.rt_period_us" ]]; then
				chown rootless:rootless "$CGROUP_MOUNT/$cg$CGROUP_PATH/cpu.rt_period_us"
			fi
			if [[ -e "$CGROUP_MOUNT/$cg$CGROUP_PATH/cpu.rt_runtime_us" ]]; then
				chown rootless:rootless "$CGROUP_MOUNT/$cg$CGROUP_PATH/cpu.rt_runtime_us"
			fi
		fi
	done
	# cgroup v2
	if [[ -e "$CGROUP_MOUNT/cgroup.controllers" ]]; then
		# Enable controllers. Some controller (e.g. memory) may fail on containerized environment.
		set -x
		# shellcheck disable=SC2013
		for f in $(cat "$CGROUP_MOUNT/cgroup.controllers"); do echo "+$f" >"$CGROUP_MOUNT/cgroup.subtree_control"; done
		set +x
		# Create the cgroup.
		mkdir -p "$CGROUP_MOUNT/$CGROUP_PATH"
		# chown/chmod dir + cgroup.subtree_control + cgroup.procs + parent's cgroup.procs.
		# See https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#delegation-containment
		chown root:rootless "$CGROUP_MOUNT/$CGROUP_PATH" "$CGROUP_MOUNT/$CGROUP_PATH/cgroup.subtree_control" "$CGROUP_MOUNT/$CGROUP_PATH/cgroup.procs" "$CGROUP_MOUNT/cgroup.procs"
		chmod g+rwx "$CGROUP_MOUNT/$CGROUP_PATH"
		chmod g+rw "$CGROUP_MOUNT/$CGROUP_PATH/cgroup.subtree_control" "$CGROUP_MOUNT/$CGROUP_PATH/cgroup.procs" "$CGROUP_MOUNT/cgroup.procs"
	fi
}

function disable_cgroup() {
	# Remove cgroups used in rootless containers.
	for cg in "${ALL_CGROUPS[@]}"; do
		[ -d "$CGROUP_MOUNT/$cg$CGROUP_PATH" ] && rmdir "$CGROUP_MOUNT/$cg$CGROUP_PATH"
	done
	# cgroup v2
	[ -d "$CGROUP_MOUNT/$CGROUP_PATH" ] && rmdir "$CGROUP_MOUNT/$CGROUP_PATH"

	return 0
}

# Create a powerset of $ALL_FEATURES (the set of all subsets of $ALL_FEATURES).
# We test all of the possible combinations (as long as we don't add too many
# feature knobs this shouldn't take too long -- but the number of tested
# combinations is O(2^n)).
function powerset() {
	eval printf '%s' "$(printf '{,%s+}' "$@")":
}
features_powerset="$(powerset "${ALL_FEATURES[@]}")"

# Make sure we have container images downloaded, as otherwise
# rootless user won't be able to write to $TESTDATA.
"$ROOT"/tests/integration/get-images.sh >/dev/null

# Iterate over the powerset of all features.
IFS=:
idx=0
for enabled_features in $features_powerset; do
	((++idx))
	printf "[%.2d] run rootless tests ... (${enabled_features%%+})\n" "$idx"

	unset IFS
	for feature in "${ALL_FEATURES[@]}"; do
		hook_func="disable_$feature"
		grep -E "(^|\+)$feature(\+|$)" <<<"$enabled_features" &>/dev/null && hook_func="enable_$feature"
		"$hook_func"
	done

	# Run the test suite!
	echo "path: $PATH"
	export ROOTLESS_FEATURES="$enabled_features"
	if [ -v RUNC_USE_SYSTEMD ]; then
		# We use `ssh rootless@localhost` instead of `sudo -u rootless` for creating systemd user session.
		# Alternatively we could use `machinectl shell`, but it is known not to work well on SELinux-enabled hosts as of April 2020:
		# https://bugzilla.redhat.com/show_bug.cgi?id=1788616
		ssh -t -t -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$HOME/rootless.key" rootless@localhost -- PATH="$PATH" RUNC_USE_SYSTEMD="$RUNC_USE_SYSTEMD" bats -t "$ROOT/tests/integration$ROOTLESS_TESTPATH"
	else
		sudo -HE -u rootless PATH="$PATH" "$(which bats)" -t "$ROOT/tests/integration$ROOTLESS_TESTPATH"
	fi
	cleanup
done
