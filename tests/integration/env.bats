#!/usr/bin/env bats
# shellcheck disable=SC2016
# This disables the check for shell variables inside single quotes
# We do that all the time in this file, as we are testing env vars.

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# Several of these tests are inspired on regressions caught by Docker, besides other tests that
# check the behavior we already had in runc:
# https://github.com/moby/moby/blob/843e51459f14ebc964d349eba1013dc8a3e9d52e/integration-cli/docker_cli_links_test.go#L197-L204
# https://github.com/moby/moby/blob/843e51459f14ebc964d349eba1013dc8a3e9d52e/integration-cli/docker_cli_run_test.go#L822-L843
#

@test "non-empty HOME env is used" {
	update_config ' .process.env += ["HOME=/override"]'
	update_config ' .process.args += ["-c", "echo $HOME"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == '/override' ]]
}

@test "empty HOME env var is overridden" {
	update_config ' .process.env += ["HOME="]'
	update_config ' .process.args += ["-c", "echo $HOME"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == '/root' ]]
}

@test "empty HOME env var is overridden with multiple overrides" {
	update_config ' .process.env += ["HOME=/override", "HOME="]'
	update_config ' .process.args += ["-c", "echo $HOME"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == '/root' ]]
}

@test "env var HOME is set only once" {
	# env will show if an env var is set multiple times.
	update_config ' .process.args = ["env"]'
	update_config ' .process.env = ["HOME=", "PATH=/usr/bin:/bin"]'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# There should be 2 words/env-vars: HOME and PATH.
	[ "$(wc -w <<<"$output")" -eq 2 ]
}

@test "env var override is set only once" {
	# env will show if an env var is set multiple times.
	update_config ' .process.args = ["env"]'
	update_config ' .process.env = ["ONE=two", "ONE=", "PATH=/usr/bin:/bin"]'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# There should be 3 words/env-vars: ONE, PATH and HOME.
	[ "$(wc -w <<<"$output")" -eq 3 ]
}

@test "env var override" {
	update_config ' .process.env += ["ONE=two", "ONE=three"]'
	update_config ' .process.args += ["-c", "echo ONE=\"$ONE\""]'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "ONE=three" ]]
}

@test "env var with new-line is honored" {
	update_config ' .process.env = ["NEW_LINE_ENV=\n", "PATH=/usr/bin:/bin"]'
	update_config ' .process.args = ["env"]'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# There should be 4 lines
	# NEW_LINE is a \n and when printed, it takes another line:
	# 1. HOME=...
	# 2. PATH=...
	# 3. NEW_LINE_ENV=
	# 4.
	#
	[ "$(wc -l <<<"$output")" -eq 4 ]
}
