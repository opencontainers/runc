#!/usr/bin/env bats

load helpers

function setup() {
	requires root apparmor
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run [unloaded apparmor profile]" {
	update_config '	  .process.apparmorProfile = "runc-test-definitely-not-loaded"'
	run ! runc run test_apparmor
	assert_output --regexp 'apparmor profile .*runc-test-definitely-not-loaded.*profile not loaded'
}
