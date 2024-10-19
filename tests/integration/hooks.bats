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

@test "runc run [hook with env]" {
	update_config '.process.args = ["/bin/true"]'
	update_config '.process.env = ["mm=nn"]'
	# All hooks except Poststop.
	for hook in prestart createRuntime createContainer startContainer poststart; do
		echo "testing hook $hook"
		# shellcheck disable=SC2016
		update_config '.hooks = {
			"'$hook'": [{
				"path": "/bin/sh",
				"args": ["/bin/sh", "-c", "[ \"$mm\"==\"tt\" ] && echo yes, we got tt from the env mm && exit 1 || exit 0"],
				"env": ["mm=tt"]
			}]
		}'
		mm=nn runc run "test_hook-$hook"
		[ "$status" -ne 0 ]
		[[ "$output" == *"yes, we got tt from the env mm"* ]]
	done
}

@test "runc run [hook without env does not inherit host env]" {
	update_config '.process.args = ["/bin/true"]'
	update_config '.process.env = ["mm=nn"]'
	# All hooks except Poststop.
	for hook in prestart createRuntime createContainer startContainer poststart; do
		echo "testing hook $hook"
		# shellcheck disable=SC2016
		update_config '.hooks = {
			"'$hook'": [{
				"path": "/bin/sh",
				"args": ["/bin/sh", "-c", "[[ \"$mm\" == \"nn\" ]] && echo \"$mm\" && exit 1 || exit 0"]
			}]
		}'
		mm=nn runc run "test_hook-$hook"
		[ "$status" -eq 0 ]
	done
}
