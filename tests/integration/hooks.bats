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

	runc ! create --console-socket "$CONSOLE_SOCKET" test_hooks
	[[ "$output" == *"error running createRuntime hook #1:"* ]]
}

@test "runc create [hook fails]" {
	for hook in prestart createRuntime createContainer; do
		echo "testing hook $hook"
		update_config '.hooks |= {"'$hook'": [{"path": "/bin/true"}, {"path": "/bin/false"}]}'
		runc ! create --console-socket "$CONSOLE_SOCKET" test_hooks
		[[ "$output" == *"error running $hook hook #1:"* ]]
	done
}

@test "runc run [hook fails]" {
	update_config '.process.args = ["/bin/echo", "Hello World"]'
	# All hooks except Poststop.
	for hook in prestart createRuntime createContainer startContainer poststart; do
		echo "testing hook $hook"
		update_config '.hooks |= {"'$hook'": [{"path": "/bin/true"}, {"path": "/bin/false"}]}'
		runc ! run "test_hook-$hook"
		[[ "$output" != "Hello World" ]]
		[[ "$output" == *"error running $hook hook #1:"* ]]
	done
}

# While runtime-spec does not say what environment variables hooks should have,
# if not explicitly specified, historically the StartContainer hook inherited
# the process environment specified for init.
#
# Check this behavior is preserved.
@test "runc run [startContainer hook should inherit process environment]" {
	cat >"rootfs/check-env.sh" <<-'EOF'
		#!/bin/sh -ue
		test $ONE = two
		test $FOO = bar
		echo $HOME # Test HOME is set w/o checking the value.
	EOF
	chmod +x "rootfs/check-env.sh"

	update_config '	  .process.args = ["/bin/true"]
			| .process.env = ["ONE=two", "FOO=bar"]
			| .hooks |= {"startContainer": [{"path": "/check-env.sh"}]}'
	runc -0 run ct1
}

# https://github.com/opencontainers/runc/issues/1663
@test "runc run [hook's argv is preserved]" {
	# Check that argv[0] and argv[1] passed to the hook's binary
	# exactly as set in config.json.
	update_config '.hooks |= {"startContainer": [{"path": "/bin/busybox", "args": ["cat", "/nosuchfile"]}]}'
	runc ! run ct1
	[[ "$output" == *"cat: can't open"*"/nosuchfile"* ]]

	# Busybox also accepts commands where argv[0] is "busybox",
	# and argv[1] is applet name. Test this as well.
	update_config '.hooks |= {"startContainer": [{"path": "/bin/busybox", "args": ["busybox", "cat", "/nosuchfile"]}]}'
	runc ! run ct1
	[[ "$output" == *"cat: can't open"*"/nosuchfile"* ]]
}
