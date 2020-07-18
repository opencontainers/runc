#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

@test "runc run [redundant default dev]" {
	update_config ' .linux.devices += [{"path": "/dev/tty", "type": "c", "major": 5, "minor": 0}]
		      | .process.args |= ["ls", "-l", "/dev/tty"]'

	runc run test_dev
	[ "$status" -eq 0 ]

	if [[ "$ROOTLESS" -ne 0 ]]; then
		[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"nobody".+"nogroup".+"5,".+"0".+"/dev/tty" ]]
	else
		[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"root".+"root".+"5,".+"0".+"/dev/tty" ]]
	fi
}
