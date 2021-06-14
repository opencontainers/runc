#!/bin/bash

# bats-core v1.2.1 defines BATS_RUN_TMPDIR
if [ -z "$BATS_RUN_TMPDIR" ]; then
	echo "bats >= v1.2.1 is required. Aborting." >&2
	exit 1
fi

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "$BASH_SOURCE")")

# Download images, get *_IMAGE variables.
IMAGES=$("${INTEGRATION_ROOT}"/get-images.sh)
eval "$IMAGES"
unset IMAGES

RUNC="${INTEGRATION_ROOT}/../../runc"
RECVTTY="${INTEGRATION_ROOT}/../../contrib/cmd/recvtty/recvtty"

# Test data path.
TESTDATA="${INTEGRATION_ROOT}/testdata"

# CRIU PATH
CRIU="$(which criu 2>/dev/null || true)"

# Kernel version
KERNEL_VERSION="$(uname -r)"
KERNEL_MAJOR="${KERNEL_VERSION%%.*}"
KERNEL_MINOR="${KERNEL_VERSION#$KERNEL_MAJOR.}"
KERNEL_MINOR="${KERNEL_MINOR%%.*}"

# Check if we're in rootless mode.
ROOTLESS=$(id -u)

# Wrapper for runc.
function runc() {
	run __runc "$@"

	# Some debug information to make life easier. bats will only print it if the
	# test failed, in which case the output is useful.
	echo "runc $@ (status=$status):" >&2
	echo "$output" >&2
}

# Raw wrapper for runc.
function __runc() {
	"$RUNC" ${RUNC_USE_SYSTEMD+--systemd-cgroup} --root "$ROOT/state" "$@"
}

# Wrapper for runc spec, which takes only one argument (the bundle path).
function runc_spec() {
	! [[ "$#" > 1 ]]

	local args=()
	local bundle=""

	if [ "$ROOTLESS" -ne 0 ]; then
		args+=("--rootless")
	fi
	if [ "$#" -ne 0 ]; then
		bundle="$1"
		args+=("--bundle" "$bundle")
	fi

	runc spec "${args[@]}"

	# Always add additional mappings if we have idmaps.
	if [[ "$ROOTLESS" -ne 0 ]] && [[ "$ROOTLESS_FEATURES" == *"idmap"* ]]; then
		runc_rootless_idmap "$bundle"
	fi
}

# Helper function to reformat config.json file. Input uses jq syntax.
function update_config() {
	bundle="${2:-.}"
	jq "$1" "$bundle/config.json" | awk 'BEGIN{RS="";getline<"-";print>ARGV[1]}' "$bundle/config.json"
}

# Shortcut to add additional uids and gids, based on the values set as part of
# a rootless configuration.
function runc_rootless_idmap() {
	bundle="${1:-.}"
	update_config ' .mounts |= map((select(.type == "devpts") | .options += ["gid=5"]) // .)
			| .linux.uidMappings += [{"hostID": '"$ROOTLESS_UIDMAP_START"', "containerID": 1000, "size": '"$ROOTLESS_UIDMAP_LENGTH"'}]
			| .linux.gidMappings += [{"hostID": '"$ROOTLESS_GIDMAP_START"', "containerID": 100, "size": 1}]
			| .linux.gidMappings += [{"hostID": '"$(($ROOTLESS_GIDMAP_START + 10))"', "containerID": 1, "size": 20}]
			| .linux.gidMappings += [{"hostID": '"$(($ROOTLESS_GIDMAP_START + 100))"', "containerID": 1000, "size": '"$(($ROOTLESS_GIDMAP_LENGTH - 1000))"'}]' $bundle
}

# Returns systemd version as a number (-1 if systemd is not enabled/supported).
function systemd_version() {
	if [ -n "${RUNC_USE_SYSTEMD}" ]; then
		systemctl --version | awk '/^systemd / {print $2; exit}'
		return
	fi

	echo "-1"
}

function init_cgroup_paths() {
	# init once
	test -n "$CGROUP_UNIFIED" && return

	if stat -f -c %t /sys/fs/cgroup | grep -qFw 63677270; then
		CGROUP_UNIFIED=yes
		local controllers="/sys/fs/cgroup/cgroup.controllers"
		# For rootless + systemd case, controllers delegation is required,
		# so check the controllers that the current user has, not the top one.
		# NOTE: delegation of cpuset requires systemd >= 244 (Fedora >= 32, Ubuntu >= 20.04).
		if [[ "$ROOTLESS" -ne 0 && -n "$RUNC_USE_SYSTEMD" ]]; then
			controllers="/sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers"
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
		CGROUP_UNIFIED=no
		CGROUP_SUBSYSTEMS=$(awk '!/^#/ {print $1}' /proc/cgroups)
		local g base_path
		for g in ${CGROUP_SUBSYSTEMS}; do
			base_path=$(gawk '$(NF-2) == "cgroup" && $NF ~ /\<'${g}'\>/ { print $5; exit }' /proc/self/mountinfo)
			test -z "$base_path" && continue
			eval CGROUP_${g^^}_BASE_PATH="${base_path}"
		done
	fi
}

# Randomize cgroup path(s), and update cgroupsPath in config.json.
# This function sets a few cgroup-related variables.
function set_cgroups_path() {
	bundle="${1:-.}"
	init_cgroup_paths

	local rnd="$RANDOM"
	if [ -n "${RUNC_USE_SYSTEMD}" ]; then
		SD_UNIT_NAME="runc-cgroups-integration-test-${rnd}.scope"
		if [ "$(id -u)" = "0" ]; then
			REL_CGROUPS_PATH="/machine.slice/$SD_UNIT_NAME"
			OCI_CGROUPS_PATH="machine.slice:runc-cgroups:integration-test-${rnd}"
		else
			REL_CGROUPS_PATH="/user.slice/user-$(id -u).slice/user@$(id -u).service/machine.slice/$SD_UNIT_NAME"
			# OCI path doesn't contain "/user.slice/user-$(id -u).slice/user@$(id -u).service/" prefix
			OCI_CGROUPS_PATH="machine.slice:runc-cgroups:integration-test-${rnd}"
		fi
	else
		REL_CGROUPS_PATH="/runc-cgroups-integration-test/test-cgroup-${rnd}"
		OCI_CGROUPS_PATH=$REL_CGROUPS_PATH
	fi

	# Absolute path to container's cgroup v2.
	if [ "$CGROUP_UNIFIED" == "yes" ]; then
		CGROUP_PATH=${CGROUP_BASE_PATH}${REL_CGROUPS_PATH}
	fi

	update_config '.linux.cgroupsPath |= "'"${OCI_CGROUPS_PATH}"'"' "$bundle"
}

# Get a value from a cgroup file.
function get_cgroup_value() {
	local source=$1
	local cgroup var current

	if [ "x$CGROUP_UNIFIED" = "xyes" ]; then
		cgroup=$CGROUP_PATH
	else
		var=${source%%.*}             # controller name (e.g. memory)
		var=CGROUP_${var^^}_BASE_PATH # variable name (e.g. CGROUP_MEMORY_BASE_PATH)
		eval cgroup=\$${var}${REL_CGROUPS_PATH}
	fi
	cat $cgroup/$source
}

# Helper to check a if value in a cgroup file matches the expected one.
function check_cgroup_value() {
	local current
	current="$(get_cgroup_value $1)"
	local expected=$2

	echo "current" $current "!?" "$expected"
	[ "$current" = "$expected" ]
}

# Helper to check a value in systemd.
function check_systemd_value() {
	[ -z "${RUNC_USE_SYSTEMD}" ] && return
	local source=$1
	[ "$source" = "unsupported" ] && return
	local expected="$2"
	local expected2="$3"
	local user=""
	[ $(id -u) != "0" ] && user="--user"

	current=$(systemctl show $user --property $source $SD_UNIT_NAME | awk -F= '{print $2}')
	echo "systemd $source: current $current !? $expected $expected2"
	[ "$current" = "$expected" ] || [ -n "$expected2" -a "$current" = "$expected2" ]
}

function check_cpu_quota() {
	local quota=$1
	local period=$2
	local sd_quota=$3

	if [ "$CGROUP_UNIFIED" = "yes" ]; then
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

	if [ "$CGROUP_UNIFIED" = "yes" ]; then
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

	check_cgroup_value "cpu.weight" $weight
	check_systemd_value "CPUWeight" $weight
}

# Helper function to set a resources limit
function set_resources_limit() {
	bundle="${1:-.}"
	update_config '.linux.resources.pids.limit |= 100' $bundle
}

# Helper function to make /sys/fs/cgroup writable
function set_cgroup_mount_writable() {
	bundle="${1:-.}"
	update_config '.mounts |= map((select(.type == "cgroup") | .options -= ["ro"]) // .)' \
		$bundle
}

# Fails the current test, providing the error given.
function fail() {
	echo "$@" >&2
	exit 1
}

# Check whether rootless runc can use cgroups.
function rootless_cgroup() {
	[[ "$ROOTLESS_FEATURES" == *"cgroup"* || -n "$RUNC_USE_SYSTEMD" ]]
}

# Allows a test to specify what things it requires. If the environment can't
# support it, the test is skipped with a message.
function requires() {
	for var in "$@"; do
		local skip_me
		case $var in
		criu)
			if [ ! -e "$CRIU" ]; then
				skip_me=1
			fi
			;;
		root)
			if [ "$ROOTLESS" -ne 0 ]; then
				skip_me=1
			fi
			;;
		rootless)
			if [ "$ROOTLESS" -eq 0 ]; then
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
			if [ "$ROOTLESS_FEATURES" != "" ]; then
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
			if [ $CGROUP_UNIFIED = "no" -a ! -e "${CGROUP_MEMORY_BASE_PATH}/memory.memsw.limit_in_bytes" ]; then
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
			if [ "$CGROUP_UNIFIED" != "no" ]; then
				skip_me=1
			fi
			;;
		cgroups_v2)
			init_cgroup_paths
			if [ "$CGROUP_UNIFIED" != "yes" ]; then
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
			local cpu_count=$(grep -c '^processor' /proc/cpuinfo)
			if [ "$cpu_count" -lt 2 ]; then
				skip_me=1
			fi
			;;
		systemd)
			if [ -z "${RUNC_USE_SYSTEMD}" ]; then
				skip_me=1
			fi
			;;
		no_systemd)
			if [ -n "${RUNC_USE_SYSTEMD}" ]; then
				skip_me=1
			fi
			;;
		*)
			fail "BUG: Invalid requires $var."
			;;
		esac
		if [ -n "$skip_me" ]; then
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
		sleep $delay
	done

	echo "Command \"$@\" failed $attempts times. Output: $output"
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
	runc state $1
	if [ $2 == "checkpointed" ]; then
		[ "$status" -eq 1 ]
		return
	fi
	[ "$status" -eq 0 ]
	[[ "${output}" == *"$2"* ]]
}

function setup_recvtty() {
	[ -z "$ROOT" ] && return 1 # must not be called without ROOT set
	local dir="$ROOT/tty"

	mkdir $dir
	export CONSOLE_SOCKET="$dir/sock"

	# We need to start recvtty in the background, so we double fork in the shell.
	("$RECVTTY" --pid-file "$dir/pid" --mode null "$CONSOLE_SOCKET" &) &
}

function teardown_recvtty() {
	[ -z "$ROOT" ] && return 0 # nothing to teardown
	local dir="$ROOT/tty"

	# When we kill recvtty, the container will also be killed.
	if [ -f "$dir/pid" ]; then
		kill -9 $(cat "$dir/pid")
	fi

	# Clean up the files that might be left over.
	rm -rf "$dir"
}

function setup_bundle() {
	local image="$1"

	# Root for various container directories (state, tty, bundle).
	export ROOT=$(mktemp -d "$BATS_RUN_TMPDIR/runc.XXXXXX")
	mkdir -p "$ROOT/state" "$ROOT/bundle/rootfs"

	setup_recvtty
	cd "$ROOT/bundle"

	tar --exclude './dev/*' -C rootfs -xf "$image"

	runc_spec
}

function setup_busybox() {
	setup_bundle "$BUSYBOX_IMAGE"
}

function setup_hello() {
	setup_bundle "$HELLO_IMAGE"
	update_config '(.. | select(.? == "sh")) |= "/hello"'
}

function setup_debian() {
	setup_bundle "$DEBIAN_IMAGE"
}

function teardown_bundle() {
	[ -z "$ROOT" ] && return 0 # nothing to teardown

	cd "$INTEGRATION_ROOT"
	teardown_recvtty
	local ct
	for ct in $(__runc list -q); do
		__runc delete -f "$ct"
	done
	rm -rf "$ROOT"
}
