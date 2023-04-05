#!/bin/bash

set -u

bats_require_minimum_version 1.5.0

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")

# Download images, get *_IMAGE variables.
IMAGES=$("${INTEGRATION_ROOT}"/get-images.sh)
eval "$IMAGES"
unset IMAGES

: "${RUNC:="${INTEGRATION_ROOT}/../../runc"}"
RECVTTY="${INTEGRATION_ROOT}/../../contrib/cmd/recvtty/recvtty"
SD_HELPER="${INTEGRATION_ROOT}/../../contrib/cmd/sd-helper/sd-helper"
SECCOMP_AGENT="${INTEGRATION_ROOT}/../../contrib/cmd/seccompagent/seccompagent"

# Some variables may not always be set. Set those to empty value,
# if unset, to avoid "unbound variable" error.
: "${ROOTLESS_FEATURES:=}"

# Test data path.
# shellcheck disable=SC2034
TESTDATA="${INTEGRATION_ROOT}/testdata"

# Kernel version
KERNEL_VERSION="$(uname -r)"
KERNEL_MAJOR="${KERNEL_VERSION%%.*}"
KERNEL_MINOR="${KERNEL_VERSION#"$KERNEL_MAJOR".}"
KERNEL_MINOR="${KERNEL_MINOR%%.*}"

ARCH=$(uname -m)

# Seccomp agent socket.
SECCCOMP_AGENT_SOCKET="$BATS_TMPDIR/seccomp-agent.sock"

# Wrapper for runc.
function runc() {
	run __runc "$@"

	# Some debug information to make life easier. bats will only print it if the
	# test failed, in which case the output is useful.
	# shellcheck disable=SC2154
	echo "$(basename "$RUNC") $* (status=$status):" >&2
	# shellcheck disable=SC2154
	echo "$output" >&2
}

# Raw wrapper for runc.
function __runc() {
	"$RUNC" ${RUNC_USE_SYSTEMD+--systemd-cgroup} \
		${ROOT:+--root "$ROOT/state"} "$@"
}

# Wrapper for runc spec.
function runc_spec() {
	local rootless=""
	[ $EUID -ne 0 ] && rootless="--rootless"

	runc spec $rootless

	# Always add additional mappings if we have idmaps.
	if [[ $EUID -ne 0 && "$ROOTLESS_FEATURES" == *"idmap"* ]]; then
		runc_rootless_idmap
	fi
}

# Helper function to reformat config.json file. Input uses jq syntax.
function update_config() {
	jq "$@" "./config.json" | awk 'BEGIN{RS="";getline<"-";print>ARGV[1]}' "./config.json"
}

# Shortcut to add additional uids and gids, based on the values set as part of
# a rootless configuration.
function runc_rootless_idmap() {
	update_config ' .mounts |= map((select(.type == "devpts") | .options += ["gid=5"]) // .)
			| .linux.uidMappings += [{"hostID": '"$ROOTLESS_UIDMAP_START"', "containerID": 1000, "size": '"$ROOTLESS_UIDMAP_LENGTH"'}]
			| .linux.gidMappings += [{"hostID": '"$ROOTLESS_GIDMAP_START"', "containerID": 100, "size": 1}]
			| .linux.gidMappings += [{"hostID": '"$((ROOTLESS_GIDMAP_START + 10))"', "containerID": 1, "size": 20}]
			| .linux.gidMappings += [{"hostID": '"$((ROOTLESS_GIDMAP_START + 100))"', "containerID": 1000, "size": '"$((ROOTLESS_GIDMAP_LENGTH - 1000))"'}]'
}

# Returns systemd version as a number (-1 if systemd is not enabled/supported).
function systemd_version() {
	if [ -v RUNC_USE_SYSTEMD ]; then
		systemctl --version | awk '/^systemd / {print $2; exit}'
		return
	fi

	echo "-1"
}

function init_cgroup_paths() {
	# init once
	[[ -v CGROUP_V1 || -v CGROUP_V2 ]] && return

	if stat -f -c %t /sys/fs/cgroup | grep -qFw 63677270; then
		CGROUP_V2=yes
		local controllers="/sys/fs/cgroup/cgroup.controllers"
		# For rootless + systemd case, controllers delegation is required,
		# so check the controllers that the current user has, not the top one.
		# NOTE: delegation of cpuset requires systemd >= 244 (Fedora >= 32, Ubuntu >= 20.04).
		if [[ $EUID -ne 0 && -v RUNC_USE_SYSTEMD ]]; then
			controllers="/sys/fs/cgroup/user.slice/user-${UID}.slice/user@${UID}.service/cgroup.controllers"
		fi

		# "pseudo" controllers do not appear in /sys/fs/cgroup/cgroup.controllers.
		# - devices (since kernel 4.15) we must assume to be supported because
		#   it's quite hard to test.
		# - freezer (since kernel 5.2) we can auto-detect by looking for the
		#   "cgroup.freeze" file a *non-root* cgroup.
		CGROUP_SUBSYSTEMS=$(
			cat "$controllers"
			echo devices
		)
		CGROUP_BASE_PATH=/sys/fs/cgroup

		# Find any cgroup.freeze files...
		if [ -n "$(find "$CGROUP_BASE_PATH" -type f -name "cgroup.freeze" -print -quit)" ]; then
			CGROUP_SUBSYSTEMS+=" freezer"
		fi
	else
		if stat -f -c %t /sys/fs/cgroup/unified 2>/dev/null | grep -qFw 63677270; then
			CGROUP_HYBRID=yes
		fi
		CGROUP_V1=yes
		CGROUP_SUBSYSTEMS=$(awk '!/^#/ {print $1}' /proc/cgroups)
		local g base_path
		for g in ${CGROUP_SUBSYSTEMS}; do
			base_path=$(awk '$(NF-2) == "cgroup" && $NF ~ /\<'"${g}"'\>/ { print $5; exit }' /proc/self/mountinfo)
			test -z "$base_path" && continue
			eval CGROUP_"${g^^}"_BASE_PATH="${base_path}"
		done
	fi
}

function create_parent() {
	if [ -v RUNC_USE_SYSTEMD ]; then
		[ ! -v SD_PARENT_NAME ] && return
		"$SD_HELPER" --parent machine.slice start "$SD_PARENT_NAME"
	else
		[ ! -v REL_PARENT_PATH ] && return
		if [ -v CGROUP_V2 ]; then
			mkdir "/sys/fs/cgroup$REL_PARENT_PATH"
		else
			local subsys
			for subsys in ${CGROUP_SUBSYSTEMS}; do
				# Have to ignore EEXIST (-p) as some subsystems
				# are mounted together (e.g. cpu,cpuacct), so
				# the path is created more than once.
				mkdir -p "/sys/fs/cgroup/$subsys$REL_PARENT_PATH"
			done
		fi
	fi
}

function remove_parent() {
	if [ -v RUNC_USE_SYSTEMD ]; then
		[ ! -v SD_PARENT_NAME ] && return
		"$SD_HELPER" --parent machine.slice stop "$SD_PARENT_NAME"
	else
		[ ! -v REL_PARENT_PATH ] && return
		if [ -v CGROUP_V2 ]; then
			rmdir "/sys/fs/cgroup/$REL_PARENT_PATH"
		else
			local subsys
			for subsys in ${CGROUP_SUBSYSTEMS} systemd; do
				rmdir "/sys/fs/cgroup/$subsys/$REL_PARENT_PATH"
			done
		fi
	fi
	unset SD_PARENT_NAME
	unset REL_PARENT_PATH
}

function set_parent_systemd_properties() {
	[ ! -v SD_PARENT_NAME ] && return
	local user=""
	[ $EUID -ne 0 ] && user="--user"
	systemctl set-property $user "$SD_PARENT_NAME" "$@"
}

# Randomize cgroup path(s), and update cgroupsPath in config.json.
# This function sets a few cgroup-related variables.
#
# Optional parameter $1 is a pod/parent name. If set, a parent/pod cgroup is
# created, and variables $REL_PARENT_PATH and $SD_PARENT_NAME can be used to
# refer to it.
function set_cgroups_path() {
	init_cgroup_paths
	local pod dash_pod="" slash_pod="" pod_slice=""
	if [ "$#" -ne 0 ] && [ "$1" != "" ]; then
		# Set up a parent/pod cgroup.
		pod="$1"
		dash_pod="-$pod"
		slash_pod="/$pod"
		SD_PARENT_NAME="machine-${pod}.slice"
		pod_slice="/$SD_PARENT_NAME"
	fi

	local rnd="$RANDOM"
	if [ -v RUNC_USE_SYSTEMD ]; then
		SD_UNIT_NAME="runc-cgroups-integration-test-${rnd}.scope"
		if [ $EUID -eq 0 ]; then
			REL_PARENT_PATH="/machine.slice${pod_slice}"
			OCI_CGROUPS_PATH="machine${dash_pod}.slice:runc-cgroups:integration-test-${rnd}"
		else
			REL_PARENT_PATH="/user.slice/user-${UID}.slice/user@${UID}.service/machine.slice${pod_slice}"
			# OCI path doesn't contain "/user.slice/user-${UID}.slice/user@${UID}.service/" prefix
			OCI_CGROUPS_PATH="machine${dash_pod}.slice:runc-cgroups:integration-test-${rnd}"
		fi
		REL_CGROUPS_PATH="$REL_PARENT_PATH/$SD_UNIT_NAME"
	else
		REL_PARENT_PATH="/runc-cgroups-integration-test${slash_pod}"
		REL_CGROUPS_PATH="$REL_PARENT_PATH/test-cgroup-${rnd}"
		OCI_CGROUPS_PATH=$REL_CGROUPS_PATH
	fi

	# Absolute path to container's cgroup v2.
	if [ -v CGROUP_V2 ]; then
		CGROUP_PATH=${CGROUP_BASE_PATH}${REL_CGROUPS_PATH}
	fi

	[ -v pod ] && create_parent

	update_config '.linux.cgroupsPath |= "'"${OCI_CGROUPS_PATH}"'"'
}

# Get a path to cgroup directory, based on controller name.
# Parameters:
#  $1: controller name (like "pids") or a file name (like "pids.max").
function get_cgroup_path() {
	if [ -v CGROUP_V2 ]; then
		echo "$CGROUP_PATH"
		return
	fi

	local var cgroup
	var=${1%%.*}                  # controller name (e.g. memory)
	var=CGROUP_${var^^}_BASE_PATH # variable name (e.g. CGROUP_MEMORY_BASE_PATH)
	eval cgroup=\$"${var}${REL_CGROUPS_PATH}"
	echo "$cgroup"
}

# Get a value from a cgroup file.
function get_cgroup_value() {
	local cgroup
	cgroup="$(get_cgroup_path "$1")"
	cat "$cgroup/$1"
}

# Helper to check a if value in a cgroup file matches the expected one.
function check_cgroup_value() {
	local current
	current="$(get_cgroup_value "$1")"
	local expected=$2

	echo "current $current !? $expected"
	[ "$current" = "$expected" ]
}

# Helper to check a value in systemd.
function check_systemd_value() {
	[ ! -v RUNC_USE_SYSTEMD ] && return
	local source="$1"
	[ "$source" = "unsupported" ] && return
	local expected="$2"
	local expected2="${3:-}"
	local user=""
	[ $EUID -ne 0 ] && user="--user"

	current=$(systemctl show $user --property "$source" "$SD_UNIT_NAME" | awk -F= '{print $2}')
	echo "systemd $source: current $current !? $expected $expected2"
	[ "$current" = "$expected" ] || [[ -n "$expected2" && "$current" = "$expected2" ]]
}

function check_cpu_quota() {
	local quota=$1
	local period=$2
	local sd_quota=$3

	if [ -v CGROUP_V2 ]; then
		if [ "$quota" = "-1" ]; then
			quota="max"
		fi
		check_cgroup_value "cpu.max" "$quota $period"
	else
		check_cgroup_value "cpu.cfs_quota_us" $quota
		check_cgroup_value "cpu.cfs_period_us" "$period"
	fi
	# systemd values are the same for v1 and v2
	check_systemd_value "CPUQuotaPerSecUSec" "$sd_quota"

	# CPUQuotaPeriodUSec requires systemd >= v242
	[ "$(systemd_version)" -lt 242 ] && return

	local sd_period=$((period / 1000))ms
	[ "$sd_period" = "1000ms" ] && sd_period="1s"
	local sd_infinity=""
	# 100ms is the default value, and if not set, shown as infinity
	[ "$sd_period" = "100ms" ] && sd_infinity="infinity"
	check_systemd_value "CPUQuotaPeriodUSec" $sd_period $sd_infinity
}

# Works for cgroup v1 and v2, accepts v1 shares as an argument.
function check_cpu_shares() {
	local shares=$1

	if [ -v CGROUP_V2 ]; then
		local weight=$((1 + ((shares - 2) * 9999) / 262142))
		check_cpu_weight "$weight"
	else
		check_cgroup_value "cpu.shares" "$shares"
		check_systemd_value "CPUShares" "$shares"
	fi
}

# Works only for cgroup v2, accept v2 weight.
function check_cpu_weight() {
	local weight=$1

	check_cgroup_value "cpu.weight" "$weight"
	check_systemd_value "CPUWeight" "$weight"
}

# Helper function to set a resources limit
function set_resources_limit() {
	update_config '.linux.resources.pids.limit |= 100'
}

# Helper function to make /sys/fs/cgroup writable
function set_cgroup_mount_writable() {
	update_config '.mounts |= map((select(.type == "cgroup") | .options -= ["ro"]) // .)'
}

# Fails the current test, providing the error given.
function fail() {
	echo "$@" >&2
	exit 1
}

# Check whether rootless runc can use cgroups.
function rootless_cgroup() {
	[[ "$ROOTLESS_FEATURES" == *"cgroup"* || -v RUNC_USE_SYSTEMD ]]
}

# Check if criu is available and working.
function have_criu() {
	command -v criu &>/dev/null || return 1

	# Workaround for https://github.com/opencontainers/runc/issues/3532.
	local ver
	ver=$(rpm -q criu 2>/dev/null || true)
	run ! grep -q '^criu-3\.17-[123]\.el9' <<<"$ver"
}

# Allows a test to specify what things it requires. If the environment can't
# support it, the test is skipped with a message.
function requires() {
	for var in "$@"; do
		local skip_me
		case $var in
		criu)
			if ! have_criu; then
				skip_me=1
			fi
			;;
		root)
			if [ $EUID -ne 0 ]; then
				skip_me=1
			fi
			;;
		rootless)
			if [ $EUID -eq 0 ]; then
				skip_me=1
			fi
			;;
		rootless_idmap)
			if [[ "$ROOTLESS_FEATURES" != *"idmap"* ]]; then
				skip_me=1
			fi
			;;
		rootless_cgroup)
			if ! rootless_cgroup; then
				skip_me=1
			fi
			;;
		rootless_no_cgroup)
			if rootless_cgroup; then
				skip_me=1
			fi
			;;
		rootless_no_features)
			if [ -n "$ROOTLESS_FEATURES" ]; then
				skip_me=1
			fi
			;;
		cgroups_rt)
			init_cgroup_paths
			if [ ! -e "${CGROUP_CPU_BASE_PATH}/cpu.rt_period_us" ]; then
				skip_me=1
			fi
			;;
		cgroups_swap)
			init_cgroup_paths
			if [ -v CGROUP_V1 ] && [ ! -e "${CGROUP_MEMORY_BASE_PATH}/memory.memsw.limit_in_bytes" ]; then
				skip_me=1
			fi
			;;
		cgroups_cpu_idle)
			local p
			init_cgroup_paths
			[ -v CGROUP_V1 ] && p="$CGROUP_CPU_BASE_PATH"
			[ -v CGROUP_V2 ] && p="$CGROUP_BASE_PATH"
			if [ -z "$(find "$p" -name cpu.idle -print -quit)" ]; then
				skip_me=1
			fi
			;;
		cgroupns)
			if [ ! -e "/proc/self/ns/cgroup" ]; then
				skip_me=1
			fi
			;;
		cgroups_v1)
			init_cgroup_paths
			if [ ! -v CGROUP_V1 ]; then
				skip_me=1
			fi
			;;
		cgroups_v2)
			init_cgroup_paths
			if [ ! -v CGROUP_V2 ]; then
				skip_me=1
			fi
			;;
		cgroups_hybrid)
			init_cgroup_paths
			if [ ! -v CGROUP_HYBRID ]; then
				skip_me=1
			fi
			;;
		cgroups_*)
			init_cgroup_paths
			var=${var#cgroups_}
			if [[ "$CGROUP_SUBSYSTEMS" != *"$var"* ]]; then
				skip_me=1
			fi
			;;
		smp)
			local cpus
			cpus=$(grep -c '^processor' /proc/cpuinfo)
			if [ "$cpus" -lt 2 ]; then
				skip_me=1
			fi
			;;
		systemd)
			if [ ! -v RUNC_USE_SYSTEMD ]; then
				skip_me=1
			fi
			;;
		no_systemd)
			if [ -v RUNC_USE_SYSTEMD ]; then
				skip_me=1
			fi
			;;
		arch_x86_64)
			if [ "$ARCH" != "x86_64" ]; then
				skip_me=1
			fi
			;;
		more_than_8_core)
			local cpus
			cpus=$(grep -c '^processor' /proc/cpuinfo)
			if [ "$cpus" -le 8 ]; then
				skip_me=1
			fi
			;;
		*)
			fail "BUG: Invalid requires $var."
			;;
		esac
		if [ -v skip_me ]; then
			skip "test requires $var"
		fi
	done
}

# Retry a command $1 times until it succeeds. Wait $2 seconds between retries.
function retry() {
	local attempts=$1
	shift
	local delay=$1
	shift
	local i

	for ((i = 0; i < attempts; i++)); do
		run "$@"
		if [[ "$status" -eq 0 ]]; then
			return 0
		fi
		sleep "$delay"
	done

	echo "Command \"$*\" failed $attempts times. Output: $output"
	false
}

# retry until the given container has state
function wait_for_container() {
	if [ $# -eq 3 ]; then
		retry "$1" "$2" __runc state "$3"
	elif [ $# -eq 4 ]; then
		retry "$1" "$2" eval "__runc state $3 | grep -qw $4"
	else
		echo "Usage: wait_for_container ATTEMPTS DELAY ID [STATUS]" 1>&2
		return 1
	fi
}

function testcontainer() {
	# test state of container
	runc state "$1"
	if [ "$2" = "checkpointed" ]; then
		[ "$status" -eq 1 ]
		return
	fi
	[ "$status" -eq 0 ]
	[[ "${output}" == *"$2"* ]]
}

function setup_recvtty() {
	[ ! -v ROOT ] && return 1 # must not be called without ROOT set
	local dir="$ROOT/tty"

	mkdir "$dir"
	export CONSOLE_SOCKET="$dir/sock"

	# We need to start recvtty in the background, so we double fork in the shell.
	("$RECVTTY" --pid-file "$dir/pid" --mode null "$CONSOLE_SOCKET" &) &
}

function teardown_recvtty() {
	[ ! -v ROOT ] && return 0 # nothing to teardown
	local dir="$ROOT/tty"

	# When we kill recvtty, the container will also be killed.
	if [ -f "$dir/pid" ]; then
		kill -9 "$(cat "$dir/pid")"
	fi

	# Clean up the files that might be left over.
	rm -rf "$dir"
}

function setup_seccompagent() {
	("${SECCOMP_AGENT}" -socketfile="$SECCCOMP_AGENT_SOCKET" -pid-file "$BATS_TMPDIR/seccompagent.pid" &) &
}

function teardown_seccompagent() {
	if [ -f "$BATS_TMPDIR/seccompagent.pid" ]; then
		kill -9 "$(cat "$BATS_TMPDIR/seccompagent.pid")"
	fi
	rm -f "$BATS_TMPDIR/seccompagent.pid"
	rm -f "$SECCCOMP_AGENT_SOCKET"
}

function setup_bundle() {
	local image="$1"

	# Root for various container directories (state, tty, bundle).
	ROOT=$(mktemp -d "$BATS_RUN_TMPDIR/runc.XXXXXX")
	mkdir -p "$ROOT/state" "$ROOT/bundle/rootfs"

	# Directories created by mktemp -d have 0700 permission bits. Tests
	# running inside userns (see userns.bats) need to access the directory
	# as a different user to mount the rootfs. Since kernel v5.12, parent
	# directories are also checked. Give a+x for these tests to work.
	chmod a+x "$ROOT" "$BATS_RUN_TMPDIR"

	setup_recvtty
	cd "$ROOT/bundle" || return

	tar --exclude './dev/*' -C rootfs -xf "$image"

	runc_spec
}

function setup_busybox() {
	setup_bundle "$BUSYBOX_IMAGE"
}

function setup_debian() {
	setup_bundle "$DEBIAN_IMAGE"
}

function teardown_bundle() {
	[ ! -v ROOT ] && return 0 # nothing to teardown

	cd "$INTEGRATION_ROOT" || return
	teardown_recvtty
	local ct
	for ct in $(__runc list -q); do
		__runc delete -f "$ct"
	done
	rm -rf "$ROOT"
	remove_parent
}

function requires_kernel() {
	local major_required minor_required
	major_required=$(echo "$1" | cut -d. -f1)
	minor_required=$(echo "$1" | cut -d. -f2)
	if [[ "$KERNEL_MAJOR" -lt $major_required || ("$KERNEL_MAJOR" -eq $major_required && "$KERNEL_MINOR" -lt $minor_required) ]]; then
		skip "requires kernel $1"
	fi
}
