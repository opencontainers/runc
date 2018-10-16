#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

@test "runc run [bind mount]" {
	CONFIG=$(jq '.mounts |= . + [{"source": ".", "destination": "/tmp/bind", "options": ["bind"]}] | .process.args = ["ls", "/tmp/bind/config.json"]' config.json)
	echo "${CONFIG}" >config.json

	runc run test_bind_mount
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" =~ '/tmp/bind/config.json' ]]
}

# make sure there is no "df: /foo/bar: No such file or directory" issue in the default configuration
@test "runc run [df]" {
	  CONFIG=$(jq '.process.args = ["df"]' config.json)
	  echo "${CONFIG}" >config.json

	  runc run test_df
	  [ "$status" -eq 0 ]
}
