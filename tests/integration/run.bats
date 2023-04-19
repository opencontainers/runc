#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/echo", "Hello World"]'
}

function teardown() {
	teardown_bundle
}

@test "runc run" {
	runc run test_hello
	[ "$status" -eq 0 ]

	runc state test_hello
	[ "$status" -ne 0 ]
}

@test "runc run --keep" {
	runc run --keep test_run_keep
	[ "$status" -eq 0 ]

	testcontainer test_run_keep stopped

	runc state test_run_keep
	[ "$status" -eq 0 ]

	runc delete test_run_keep

	runc state test_run_keep
	[ "$status" -ne 0 ]
}

@test "runc run --keep (check cgroup exists)" {
	# for systemd driver, the unit's cgroup path will be auto removed if container's all processes exited
	requires no_systemd
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc run --keep test_run_keep
	[ "$status" -eq 0 ]

	testcontainer test_run_keep stopped

	runc state test_run_keep
	[ "$status" -eq 0 ]

	# check that cgroup exists
	check_cgroup_value "pids.max" "max"

	runc delete test_run_keep

	runc state test_run_keep
	[ "$status" -ne 0 ]
}

@test "runc run [hostname domainname]" {
	update_config ' .process.args |= ["sh"]
			| .hostname = "myhostname"
			| .domainname= "mydomainname"'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_utc
	[ "$status" -eq 0 ]

	# test hostname
	runc exec test_utc hostname
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *'myhostname'* ]]

	# test domainname
	runc exec test_utc cat /proc/sys/kernel/domainname
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *'mydomainname'* ]]
}
