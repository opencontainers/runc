#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
	teardown_running_container testbusyboxdelete
}

@test "runc delete" {
	runc run -d --console-socket "$CONSOLE_SOCKET" testbusyboxdelete
	[ "$status" -eq 0 ]

	testcontainer testbusyboxdelete running

	runc kill testbusyboxdelete KILL
	[ "$status" -eq 0 ]
	retry 10 1 eval "__runc state testbusyboxdelete | grep -q 'stopped'"

	runc delete testbusyboxdelete
	[ "$status" -eq 0 ]

	runc state testbusyboxdelete
	[ "$status" -ne 0 ]

	output=$(find /sys/fs/cgroup -wholename '*testbusyboxdelete*' -type d)
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

@test "runc delete --force in cgroupv1 with subcgroups" {
	requires cgroups_v1 root cgroupns
	set_cgroups_path "$BUSYBOX_BUNDLE"
	set_cgroup_mount_writable "$BUSYBOX_BUNDLE"
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
		name=CGROUP_${s^^}
		eval path=\$"${name}"/foo
		# shellcheck disable=SC2154
		[ -d "${path}" ] || fail "test failed to create memory sub-cgroup ($path not found)"
	done

	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]

	output=$(find /sys/fs/cgroup -wholename '*testbusyboxdelete*' -type d)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force in cgroupv2 with subcgroups" {
	requires cgroups_v2 root
	set_cgroups_path "$BUSYBOX_BUNDLE"
	set_cgroup_mount_writable "$BUSYBOX_BUNDLE"

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
	[ -d "$CGROUP_PATH"/foo ]

	# force delete test_busybox
	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]

	# check delete subcgroups success
	[ ! -d "$CGROUP_PATH"/foo ]
}
