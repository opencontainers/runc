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

# Issue 4047, case "runc delete".
@test "runc delete [host pidns + init gone]" {
	test_runc_delete_host_pidns
}

# Issue 4047, case "runc delete --force" (different code path).
# shellcheck disable=SC2030
@test "runc delete --force [host pidns + init gone]" {
	test_runc_delete_host_pidns --force
}

# See also: "kill KILL [host pidns + init gone]" test in kill.bats.
function test_runc_delete_host_pidns() {
	requires cgroups_freezer

	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	set_cgroups_path
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		# Apparently, for rootless test, when using systemd cgroup manager,
		# newer versions of systemd clean up the container as soon as its init
		# process is gone. This is all fine and dandy, except it prevents us to
		# test this case, thus we skip the test.
		#
		# It is not entirely clear which systemd version got this feature:
		# v245 works fine, and v249 does not.
		if [ -v RUNC_USE_SYSTEMD ] && [ "$(systemd_version)" -gt 245 ]; then
			skip "rootless+systemd conflicts with systemd > 245"
		fi
		# Can't mount real /proc when rootless + no pidns,
		# so change it to a bind-mounted one from the host.
		update_config '	  .mounts |= map((select(.type == "proc")
					| .type = "none"
					| .source = "/proc"
					| .options = ["rbind", "nosuid", "nodev", "noexec"]
				  ) // .)'
	fi

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	# shellcheck disable=SC2031
	[ "$status" -eq 0 ]
	cgpath=$(get_cgroup_path "pids")
	init_pid=$(cat "$cgpath"/cgroup.procs)

	# Start a few more processes.
	for _ in 1 2 3 4 5; do
		__runc exec -d test_busybox sleep 1h
	done

	# Now kill the container's init process. Since the container do
	# not have own PID ns, its init is no special and the container
	# will still be up and running.
	kill -9 "$init_pid"

	# Get the list of all container processes.
	pids=$(cat "$cgpath"/cgroup.procs)
	echo "pids: $pids"
	# Sanity check -- make sure all processes exist.
	for p in $pids; do
		kill -0 "$p"
	done

	# Must kill those processes and remove container.
	# shellcheck disable=SC2031
	runc delete "$@" test_busybox
	# shellcheck disable=SC2031
	[ "$status" -eq 0 ]

	runc state test_busybox
	# shellcheck disable=SC2031
	[ "$status" -ne 0 ] # "Container does not exist"

	# Make sure all processes are gone.
	pids=$(cat "$cgpath"/cgroup.procs) || true # OK if cgroup is gone
	echo "pids: $pids"
	[ -z "$pids" ]
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
