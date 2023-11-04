#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# shellcheck disable=SC2030
@test "kill detached busybox" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# check state
	testcontainer test_busybox running

	runc kill test_busybox KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 test_busybox stopped

	# Check that kill errors on a stopped container.
	runc kill test_busybox 0
	[ "$status" -ne 0 ]
	[[ "$output" == *"container not running"* ]]

	# Check that -a (now obsoleted) makes kill return no error for a stopped container.
	runc kill -a test_busybox 0
	[ "$status" -eq 0 ]

	runc delete test_busybox
	[ "$status" -eq 0 ]
}

test_host_pidns_kill() {
	requires cgroups_freezer

	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	set_cgroups_path
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
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

	if [ -v KILL_INIT ]; then
		# Now kill the container's init process. Since the container do
		# not have own PID ns, its init is no special and the container
		# will still be up and running (except for rootless container
		# AND systemd cgroup driver AND systemd > v245, when systemd
		# kills the container; see "kill KILL [host pidns + init gone]"
		# below).
		kill -9 "$init_pid"
	fi

	# Get the list of all container processes.
	pids=$(cat "$cgpath"/cgroup.procs)
	echo "pids: $pids"
	# Sanity check -- make sure all processes exist.
	for p in $pids; do
		kill -0 "$p"
	done

	runc kill test_busybox KILL
	# shellcheck disable=SC2031
	[ "$status" -eq 0 ]
	wait_for_container 10 1 test_busybox stopped

	# Make sure all processes are gone.
	pids=$(cat "$cgpath"/cgroup.procs) || true # OK if cgroup is gone
	echo "pids: $pids"
	[ -z "$pids" ]
}

# This is roughly the same as TestPIDHostInitProcessWait in libcontainer/integration.
# The differences are:
#
# 1. Here we use separate processes to create and to kill a container, so the
#    processes inside a container are not children of "runc kill".
#
# 2. We hit different codepaths (nonChildProcess.signal rather than initProcess.signal).
@test "kill KILL [host pidns]" {
	unset KILL_INIT
	test_host_pidns_kill
}

# Same as above plus:
#
# 3. Test runc kill on a container whose init process is gone.
#
# Issue 4047, case "runc kill".
# See also: "runc delete --force [host pidns + init gone]" test in delete.bats.
@test "kill KILL [host pidns + init gone]" {
	# Apparently, for rootless test, when using systemd cgroup manager,
	# newer versions of systemd clean up the container as soon as its init
	# process is gone. This is all fine and dandy, except it prevents us to
	# test this case, thus we skip the test.
	#
	# It is not entirely clear which systemd version got this feature:
	# v245 works fine, and v249 does not.
	if [ $EUID -ne 0 ] && [ -v RUNC_USE_SYSTEMD ] && [ "$(systemd_version)" -gt 245 ]; then
		skip "rootless+systemd conflicts with systemd > 245"
	fi
	KILL_INIT=1
	test_host_pidns_kill
	unset KILL_INIT
}
