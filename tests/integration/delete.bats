#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# See also: "kill KILL [host pidns + init gone]" test in kill.bats.
#
# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
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

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox
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
	wait_pids_gone 10 0.2 "$init_pid"

	# Get the list of all container processes.
	mapfile -t pids < <(cat "$cgpath"/cgroup.procs)
	echo "pids:" "${pids[@]}"
	# Sanity check -- make sure all processes exist.
	for p in "${pids[@]}"; do
		kill -0 "$p"
	done

	# Must kill those processes and remove container.
	runc -0 delete "$@" test_busybox

	runc ! state test_busybox
	[[ "$output" == *"container does not exist"* ]]

	# Wait and check that all the processes are gone.
	wait_pids_gone 10 0.2 "${pids[@]}"

	# Make sure cgroup.procs is empty.
	mapfile -t pids < <(cat "$cgpath"/cgroup.procs || true)
	if [ ${#pids[@]} -gt 0 ]; then
		echo "expected empty cgroup.procs, got:" "${pids[@]}" 1>&2
		return 1
	fi
}

@test "runc delete" {
	# Need a permission to create a cgroup.
	# XXX(@kolyshkin): currently this test does not handle rootless when
	# fs cgroup driver is used, because in this case cgroup (with a
	# predefined name) is created by tests/rootless.sh, not by runc.
	[ $EUID -ne 0 ] && requires systemd
	set_resources_limit

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" testbusyboxdelete

	testcontainer testbusyboxdelete running
	# Ensure the find statement used later is correct.
	output=$(find /sys/fs/cgroup -name testbusyboxdelete -o -name \*-testbusyboxdelete.scope 2>/dev/null || true)
	if [ -z "$output" ]; then
		fail "expected cgroup not found"
	fi

	runc -0 kill testbusyboxdelete KILL
	wait_for_container 10 1 testbusyboxdelete stopped

	runc -0 delete testbusyboxdelete

	runc ! state testbusyboxdelete

	output=$(find /sys/fs/cgroup -name testbusyboxdelete -o -name \*-testbusyboxdelete.scope 2>/dev/null || true)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc delete --force test_busybox

	runc ! state test_busybox
}

@test "runc delete --force ignore not exist" {
	runc -0 delete --force notexists
}

# Issue 4047, case "runc delete".
@test "runc delete [host pidns + init gone]" {
	test_runc_delete_host_pidns
}

# Issue 4047, case "runc delete --force" (different code path).
@test "runc delete --force [host pidns + init gone]" {
	test_runc_delete_host_pidns --force
}

@test "runc delete --force [paused container]" {
	requires cgroups_freezer
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		# Rootless containers have no default cgroup path.
		set_cgroups_path
	fi

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" ct1
	testcontainer ct1 running

	runc -0 pause ct1
	runc -0 delete --force ct1
}

@test "runc delete --force in cgroupv1 with subcgroups" {
	requires cgroups_v1 root cgroupns
	set_cgroups_path
	set_cgroup_mount_writable
	# enable cgroupns
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	local subsystems="memory freezer"

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	__runc exec -d test_busybox sleep 1d

	# find the pid of sleep
	pid=$(__runc exec test_busybox ps -a | grep 1d | awk '{print $1}')
	[[ ${pid} =~ [0-9]+ ]]

	# create a sub-cgroup
	cat <<EOF | runc -0 exec test_busybox sh
set -e -u -x
for s in ${subsystems}; do
  cd /sys/fs/cgroup/\$s
  mkdir foo
  cd foo
  echo ${pid} > tasks
  cat tasks
done
EOF
	[[ "$output" =~ [0-9]+ ]]

	for s in ${subsystems}; do
		name=CGROUP_${s^^}_BASE_PATH
		eval path=\$"${name}${REL_CGROUPS_PATH}/foo"
		# shellcheck disable=SC2154
		[ -d "${path}" ] || fail "test failed to create memory sub-cgroup ($path not found)"
	done

	runc delete --force test_busybox

	runc ! state test_busybox

	output=$(find /sys/fs/cgroup -wholename '*testbusyboxdelete*' -type d 2>/dev/null || true)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force in cgroupv2 with subcgroups" {
	requires cgroups_v2 root
	set_cgroups_path
	set_cgroup_mount_writable

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

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
	runc -0 exec test_busybox sh <nest.sh
	[[ "$output" =~ [0-9]+ ]]

	# check create subcgroups success
	[ -d "$CGROUP_V2_PATH"/foo ]

	# force delete test_busybox
	runc delete --force test_busybox

	runc ! state test_busybox

	# check delete subcgroups success
	[ ! -d "$CGROUP_V2_PATH"/foo ]
}

@test "runc delete removes failed systemd unit" {
	requires systemd_v244 # Older systemd lacks RuntimeMaxSec support.

	set_cgroups_path
	update_config '	  .annotations += {
				"org.systemd.property.RuntimeMaxSec": "2",
				"org.systemd.property.TimeoutStopSec": "1"
			   }
			| .process.args |= ["/bin/sleep", "10"]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test-failed-unit

	wait_for_container 10 1 test-failed-unit stopped

	local user=""
	[ $EUID -ne 0 ] && user="--user"

	# Expect "unit is not active" exit code.
	run -3 systemctl status $user "$SD_UNIT_NAME"

	runc delete test-failed-unit
	# Expect "no such unit" exit code.
	run -4 systemctl status $user "$SD_UNIT_NAME"
}
