#!/usr/bin/env bats

load helpers

function setup() {
	requires arch_x86_64
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run personality for i686" {
	update_config '
      .process.args = ["/bin/sh", "-c", "uname -a"]
			| .linux.personality = {
                "domain": "LINUX32",
                "flags": []
			}'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"i686"* ]]
}

@test "runc run personality with exec for i686" {
	update_config '
      .linux.personality = {
                "domain": "LINUX32",
      }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	runc exec test_busybox /bin/sh -c "uname -a"
	[ "$status" -eq 0 ]
	[[ "$output" == *"i686"* ]]
}

@test "runc run personality for x86_64" {
	update_config '
      .process.args = ["/bin/sh", "-c", "uname -a"]
			| .linux.personality = {
                "domain": "LINUX",
                "flags": []
			}'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"x86_64"* ]]
}

@test "runc run personality with exec for x86_64" {
	update_config '
      .linux.personality = {
                "domain": "LINUX",
      }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	runc exec test_busybox /bin/sh -c "uname -a"
	[ "$status" -eq 0 ]
	[[ "$output" == *"x86_64"* ]]
}
