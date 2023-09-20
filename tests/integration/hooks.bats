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
