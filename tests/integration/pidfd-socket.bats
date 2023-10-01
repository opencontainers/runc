#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	requires_kernel 5.3
	setup_busybox
	update_config '.process.args = ["/bin/sleep", "1d"]'
}

function teardown() {
	teardown_pidfd_kill
	teardown_bundle
}

@test "runc create [ --pidfd-socket ] " {
	setup_pidfd_kill "SIGTERM"

	runc create --console-socket "$CONSOLE_SOCKET" --pidfd-socket "${PIDFD_SOCKET}" test_pidfd
	[ "$status" -eq 0 ]
	testcontainer test_pidfd created

	pidfd_kill
	wait_for_container 10 1 test_pidfd stopped
}

@test "runc run [ --pidfd-socket ] " {
	setup_pidfd_kill "SIGKILL"

	runc run -d --console-socket "$CONSOLE_SOCKET" --pidfd-socket "${PIDFD_SOCKET}" test_pidfd
	[ "$status" -eq 0 ]
	testcontainer test_pidfd running

	pidfd_kill
	wait_for_container 10 1 test_pidfd stopped
}

@test "runc exec [ --pidfd-socket ] [cgroups_v1] " {
	requires cgroups_v1

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" test_pidfd
	[ "$status" -eq 0 ]
	testcontainer test_pidfd running

	# Use sub-cgroup to ensure that exec process has been killed
	test_pidfd_cgroup_path=$(get_cgroup_path "pids")
	mkdir "${test_pidfd_cgroup_path}/exec_pidfd"
	[ "$status" -eq 0 ]

	setup_pidfd_kill "SIGKILL"

	__runc exec -d --cgroup "pids:exec_pidfd" --pid-file "exec_pid.txt" --pidfd-socket "${PIDFD_SOCKET}" test_pidfd sleep 1d
	[ "$status" -eq 0 ]

	exec_pid=$(cat exec_pid.txt)
	exec_pid_in_cgroup=$(cat "${test_pidfd_cgroup_path}/exec_pidfd/cgroup.procs")
	[ "${exec_pid}" -eq "${exec_pid_in_cgroup}" ]

	pidfd_kill

	# ensure exec process has been reaped
	retry 10 1 rmdir "${test_pidfd_cgroup_path}/exec_pidfd"

	testcontainer test_pidfd running
}

@test "runc exec [ --pidfd-socket ] [cgroups_v2] " {
	requires cgroups_v2

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" test_pidfd
	[ "$status" -eq 0 ]
	testcontainer test_pidfd running

	# Use sub-cgroup to ensure that exec process has been killed
	test_pidfd_cgroup_path=$(get_cgroup_path "pids")
	mkdir "${test_pidfd_cgroup_path}/exec_pidfd"
	[ "$status" -eq 0 ]

	setup_pidfd_kill "SIGKILL"

	__runc exec -d --cgroup "exec_pidfd" --pid-file "exec_pid.txt" --pidfd-socket "${PIDFD_SOCKET}" test_pidfd sleep 1d
	[ "$status" -eq 0 ]

	exec_pid=$(cat exec_pid.txt)
	exec_pid_in_cgroup=$(cat "${test_pidfd_cgroup_path}/exec_pidfd/cgroup.procs")
	[ "${exec_pid}" -eq "${exec_pid_in_cgroup}" ]

	pidfd_kill

	# ensure exec process has been reaped
	retry 10 1 rmdir "${test_pidfd_cgroup_path}/exec_pidfd"

	testcontainer test_pidfd running
}
