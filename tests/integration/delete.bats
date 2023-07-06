#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc delete" {
	# Need a permission to create a cgroup.
	# XXX(@kolyshkin): currently this test does not handle rootless when
	# fs cgroup driver is used, because in this case cgroup (with a
	# predefined name) is created by tests/rootless.sh, not by runc.
	[ $EUID -ne 0 ] && requires systemd
	set_resources_limit

	runc run -d --console-socket "$CONSOLE_SOCKET" testbusyboxdelete
	[ "$status" -eq 0 ]

	testcontainer testbusyboxdelete running
	# Ensure the find statement used later is correct.
	output=$(find /sys/fs/cgroup -name testbusyboxdelete -o -name \*-testbusyboxdelete.scope 2>/dev/null || true)
	if [ -z "$output" ]; then
		fail "expected cgroup not found"
	fi

	runc kill testbusyboxdelete KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 testbusyboxdelete stopped

	runc delete testbusyboxdelete
	[ "$status" -eq 0 ]

	runc state testbusyboxdelete
	[ "$status" -ne 0 ]

	output=$(find /sys/fs/cgroup -name testbusyboxdelete -o -name \*-testbusyboxdelete.scope 2>/dev/null || true)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# check state
	testcontainer test_busybox running

	# force delete test_busybox
	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]
}

@test "runc delete --force ignore not exist" {
	runc delete --force notexists
	[ "$status" -eq 0 ]
}

@test "runc delete --force [paused container]" {
	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]
	testcontainer ct1 running

	runc pause ct1
	runc delete --force ct1
	[ "$status" -eq 0 ]
}

@test "runc delete --force in cgroupv1 with subcgroups" {
	requires cgroups_v1 root cgroupns
	set_cgroups_path
	set_cgroup_mount_writable
	# enable cgroupns
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	local subsystems="memory freezer"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	__runc exec -d test_busybox sleep 1d

	# find the pid of sleep
	pid=$(__runc exec test_busybox ps -a | grep 1d | awk '{print $1}')
	[[ ${pid} =~ [0-9]+ ]]

	# create a sub-cgroup
	cat <<EOF | runc exec test_busybox sh
set -e -u -x
for s in ${subsystems}; do
  cd /sys/fs/cgroup/\$s
  mkdir foo
  cd foo
  echo ${pid} > tasks
  cat tasks
done
EOF
	[ "$status" -eq 0 ]
	[[ "$output" =~ [0-9]+ ]]

	for s in ${subsystems}; do
		name=CGROUP_${s^^}_BASE_PATH
		eval path=\$"${name}${REL_CGROUPS_PATH}/foo"
		# shellcheck disable=SC2154
		[ -d "${path}" ] || fail "test failed to create memory sub-cgroup ($path not found)"
	done

	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]

	output=$(find /sys/fs/cgroup -wholename '*testbusyboxdelete*' -type d 2>/dev/null || true)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force in cgroupv2 with subcgroups" {
	requires cgroups_v2 root
	set_cgroups_path
	set_cgroup_mount_writable

	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# check state
	testcontainer test_busybox running

	# create a sub process
	__runc exec -d test_busybox sleep 1d

	# find the pid of sleep
	pid=$(__runc exec test_busybox ps -a | grep 1d | awk '{print $1}')
	[[ ${pid} =~ [0-9]+ ]]

	# create subcgroups
	cat <<EOF >nest.sh
  set -e -u -x
  cd /sys/fs/cgroup
  echo +pids > cgroup.subtree_control
  mkdir foo
  cd foo
  echo threaded > cgroup.type
  echo ${pid} > cgroup.threads
  cat cgroup.threads
EOF
	runc exec test_busybox sh <nest.sh
	[ "$status" -eq 0 ]
	[[ "$output" =~ [0-9]+ ]]

	# check create subcgroups success
	[ -d "$CGROUP_V2_PATH"/foo ]

	# force delete test_busybox
	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]

	# check delete subcgroups success
	[ ! -d "$CGROUP_V2_PATH"/foo ]
}

@test "runc delete removes failed systemd unit" {
	requires systemd_v244 # Older systemd lacks RuntimeMaxSec support.

	set_cgroups_path
	# shellcheck disable=SC2016
	update_config '	  .annotations += {
				"org.systemd.property.RuntimeMaxSec": "2",
				"org.systemd.property.TimeoutStopSec": "1"
			   }
			| .process.args |= ["/bin/sleep", "10"]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test-failed-unit
	[ "$status" -eq 0 ]

	wait_for_container 10 1 test-failed-unit stopped

	local user=""
	[ $EUID -ne 0 ] && user="--user"

	# Expect "unit is not active" exit code.
	run -3 systemctl status $user "$SD_UNIT_NAME"

	runc delete test-failed-unit
	# Expect "no such unit" exit code.
	run -4 systemctl status $user "$SD_UNIT_NAME"
}
