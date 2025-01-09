#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc create [second createRuntime hook fails]" {
	update_config '.hooks |= {"createRuntime": [{"path": "/bin/true"}, {"path": "/bin/false"}]}'

	runc create --console-socket "$CONSOLE_SOCKET" test_hooks
	[ "$status" -ne 0 ]
	[[ "$output" == *"error running createRuntime hook #1:"* ]]
}

@test "runc create [hook fails]" {
	for hook in prestart createRuntime createContainer; do
		echo "testing hook $hook"
		# shellcheck disable=SC2016
		update_config '.hooks |= {"'$hook'": [{"path": "/bin/true"}, {"path": "/bin/false"}]}'
		runc create --console-socket "$CONSOLE_SOCKET" test_hooks
		[ "$status" -ne 0 ]
		[[ "$output" == *"error running $hook hook #1:"* ]]
	done
}

@test "runc run [hook fails]" {
	update_config '.process.args = ["/bin/echo", "Hello World"]'
	# All hooks except Poststop.
	for hook in prestart createRuntime createContainer startContainer poststart; do
		echo "testing hook $hook"
		# shellcheck disable=SC2016
		update_config '.hooks |= {"'$hook'": [{"path": "/bin/true"}, {"path": "/bin/false"}]}'
		runc run "test_hook-$hook"
		[[ "$output" != "Hello World" ]]
		[ "$status" -ne 0 ]
		[[ "$output" == *"error running $hook hook #1:"* ]]
	done
}

@test "runc run [hook with env property]" {
	update_config '.process.args = ["/bin/true"]'
	update_config '.process.env = ["TEST_VAR=val"]'
	# All hooks except Poststop.
	for hook in prestart createRuntime createContainer startContainer poststart; do
		echo "testing hook $hook"
		# shellcheck disable=SC2016
		update_config '.hooks = {
			"'$hook'": [{
				"path": "/bin/sh",
				"args": ["/bin/sh", "-c", "[ \"$TEST_VAR\"==\"val\" ] && echo yes, we got val from the env TEST_VAR && exit 1 || exit 0"],
				"env": ["TEST_VAR=val"]
			}]
		}'
		TEST_VAR="val" runc run "test_hook-$hook"
		[ "$status" -ne 0 ]
		[[ "$output" == *"yes, we got val from the env TEST_VAR"* ]]
	done
}

# https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#posix-platform-hooks
@test "runc run [hook without env property should not inherit host env]" {
	update_config '.process.args = ["/bin/true"]'
	update_config '.process.env = ["TEST_VAR=val"]'
	# All hooks except Poststop.
	for hook in prestart createRuntime createContainer startContainer poststart; do
		echo "testing hook $hook"
		# shellcheck disable=SC2016
		update_config '.hooks = {
			"'$hook'": [{
				"path": "/bin/sh",
				"args": ["/bin/sh", "-c", "[[ \"$TEST_VAR\" == \"val\" ]] && echo \"$TEST_VAR\" && exit 1 || exit 0"]
			}]
		}'
		TEST_VAR="val" runc run "test_hook-$hook"
		[ "$status" -eq 0 ]
	done
}
