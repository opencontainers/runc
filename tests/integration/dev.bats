#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

@test "runc run [redundant default /dev/tty]" {
	update_config ' .linux.devices += [{"path": "/dev/tty", "type": "c", "major": 5, "minor": 0}]
		      | .process.args |= ["ls", "-lLn", "/dev/tty"]'

	runc run test_dev
	[ "$status" -eq 0 ]

	if [[ "$ROOTLESS" -ne 0 ]]; then
		[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"65534".+"65534".+"5,".+"0".+"/dev/tty" ]]
	else
		[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"0".+"0".+"5,".+"0".+"/dev/tty" ]]
	fi
}

@test "runc run [redundant default /dev/ptmx]" {
	update_config ' .linux.devices += [{"path": "/dev/ptmx", "type": "c", "major": 5, "minor": 2}]
		      | .process.args |= ["ls", "-lLn", "/dev/ptmx"]'

	runc run test_dev
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"0".+"0".+"5,".+"2".+"/dev/ptmx" ]]
}
