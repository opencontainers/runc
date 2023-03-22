#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	id=test_busybox
	#id=ct-${RANDOM}
	#	if [ -v RUNC_USE_SYSTEMD ]; then
	#		update_config '.linux.cgroupsPath |= ":runc:test_busybox-'${RANDOM}'"'
	#	fi
}

function teardown() {
	teardown_bundle
}

@test "ps" {
	# ps is not supported, it requires cgroups
	requires root

	systemctl status runc-test_busybox.scope || true
	runc run -d --console-socket "$CONSOLE_SOCKET" $id
	[ "$status" -eq 0 ]

	testcontainer $id running

	runc ps $id
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ UID\ +PID\ +PPID\ +C\ +STIME\ +TTY\ +TIME\ +CMD+ ]]
	[[ "${lines[1]}" == *"$(id -un 2>/dev/null)"*[0-9]* ]]
}

@test "ps -f json" {
	# ps is not supported, it requires cgroups
	requires root

	systemctl status runc-test_busybox.scope || true
	runc run -d --console-socket "$CONSOLE_SOCKET" $id
	[ "$status" -eq 0 ]

	testcontainer $id running

	runc ps -f json $id
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ [0-9]+ ]]
}

@test "ps -e -x" {
	# ps is not supported, it requires cgroups
	requires root

	systemctl status runc-test_busybox.scope || true
	runc run -d --console-socket "$CONSOLE_SOCKET" $id
	[ "$status" -eq 0 ]

	testcontainer $id running

	runc ps $id -e -x
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ \ +PID\ +TTY\ +STAT\ +TIME\ +COMMAND+ ]]
	[[ "${lines[1]}" =~ [0-9]+ ]]
}

@test "ps after the container stopped" {
	# ps requires cgroups
	[ $EUID -ne 0 ] && requires rootless_cgroup
	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" $id
	[ "$status" -eq 0 ]

	testcontainer $id running

	runc ps $id
	[ "$status" -eq 0 ]

	runc kill $id KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 $id stopped

	runc ps $id
	[ "$status" -eq 0 ]
}
