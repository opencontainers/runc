#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
	teardown_running_container test_bind_mount
}

@test "runc run [bind mount]" {
	update_config ' .mounts += [{"source": ".", "destination": "/tmp/bind", "options": ["bind"]}]
			| .process.args |= ["ls", "/tmp/bind/config.json"]'

	runc run test_bind_mount
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *'/tmp/bind/config.json'* ]]
}

@test "runc run [ro tmpfs mount]" {
	update_config 	' .mounts += [{"source": "tmpfs", "destination": "/mnt", "type": "tmpfs", "options": ["ro", "nodev", "nosuid", "mode=755"]}] 
			| .process.args |= ["grep", "^tmpfs /mnt", "/proc/mounts"]' 

	runc run test_ro_tmpfs_mount
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *'ro,'* ]]
}
