#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/echo", "Hello World"]'
}

function teardown() {
	teardown_bundle
}

function check_debug() {
	[[ "$*" == *"nsexec container setup"* ]]
	[[ "$*" == *"child process in init()"* ]]
	[[ "$*" == *"init: closing the pipe to signal completion"* ]]
}

@test "global --debug" {
	run -0 runc --debug run test_hello

	# check expected debug output was sent to stderr
	[[ "${output}" == *"level=debug"* ]]
	check_debug "$output"
}

@test "global --debug to --log" {
	run -0 runc --log log.out --debug run test_hello

	# check output does not include debug info
	[[ "${output}" != *"level=debug"* ]]

	cat log.out >&2
	# check expected debug output was sent to log.out
	output=$(cat log.out)
	[[ "${output}" == *"level=debug"* ]]
	check_debug "$output"
}

@test "global --debug to --log --log-format 'text'" {
	run -0 runc --log log.out --log-format "text" --debug run test_hello

	# check output does not include debug info
	[[ "${output}" != *"level=debug"* ]]

	cat log.out >&2
	# check expected debug output was sent to log.out
	output=$(cat log.out)
	[[ "${output}" == *"level=debug"* ]]
	check_debug "$output"
}

@test "global --debug to --log --log-format 'json'" {
	run -0 runc --log log.out --log-format "json" --debug run test_hello

	# check output does not include debug info
	[[ "${output}" != *"level=debug"* ]]

	cat log.out >&2
	# check expected debug output was sent to log.out
	output=$(cat log.out)
	[[ "${output}" == *'"level":"debug"'* ]]
	check_debug "$output"
}
