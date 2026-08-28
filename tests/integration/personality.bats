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

	run -0 runc run test_busybox
	[[ "$output" == *"i686"* ]]
}

@test "runc run personality with exec for i686" {
	update_config '
      .linux.personality = {
                "domain": "LINUX32",
      }'

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	run -0 runc exec test_busybox /bin/sh -c "uname -a"
	[[ "$output" == *"i686"* ]]
}

@test "runc run personality for x86_64" {
	update_config '
      .process.args = ["/bin/sh", "-c", "uname -a"]
			| .linux.personality = {
                "domain": "LINUX",
                "flags": []
			}'

	run -0 runc run test_busybox
	[[ "$output" == *"x86_64"* ]]
}

@test "runc run personality with exec for x86_64" {
	update_config '
      .linux.personality = {
                "domain": "LINUX",
      }'

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	run -0 runc exec test_busybox /bin/sh -c "uname -a"
	[[ "$output" == *"x86_64"* ]]
}

# check that personality can be set when the personality syscall is blocked by seccomp
@test "runc run with personality syscall blocked by seccomp" {
	update_config '
      .linux.personality = {
                "domain": "LINUX",
      }
	  | .linux.seccomp = {
                "defaultAction":"SCMP_ACT_ALLOW",
                "syscalls":[{"names":["personality"], "action":"SCMP_ACT_ERRNO"}]
	  }'

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	run -0 runc exec test_busybox /bin/sh -c "uname -a"
	[[ "$output" == *"x86_64"* ]]
}
