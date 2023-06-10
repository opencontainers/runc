#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

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

# This is roughly the same as TestPIDHostInitProcessWait in libcontainer/integration.
@test "kill KILL [host pidns]" {
	# kill -a currently requires cgroup freezer.
	requires cgroups_freezer

	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	set_cgroups_path
	if [ $EUID -ne 0 ]; then
		# kill --all requires working cgroups.
		requires rootless_cgroup
		update_config '	  .mounts |= map((select(.type == "proc")
					| .type = "none"
					| .source = "/proc"
					| .options = ["rbind", "nosuid", "nodev", "noexec"]
				  ) // .)'
	fi

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# Start a few more processes.
	for _ in 1 2 3 4 5; do
		__runc exec -d test_busybox sleep 1h
		[ "$status" -eq 0 ]
	done

	# Get the list of all container processes.
	path=$(get_cgroup_path "pids")
	pids=$(cat "$path"/cgroup.procs)
	echo "pids: $pids"
	# Sanity check -- make sure all processes exist.
	for p in $pids; do
		kill -0 "$p"
	done

	runc kill test_busybox KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 test_busybox stopped

	# Make sure all processes are gone.
	pids=$(cat "$path"/cgroup.procs) || true # OK if cgroup is gone
	echo "pids: $pids"
	for p in $pids; do
		run ! kill -0 "$p"
		[[ "$output" = *"No such process" ]]
	done
}
