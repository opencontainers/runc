#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run --no-pivot must not expose bare /proc" {
	requires root

	update_config '	  .process.args |= ["unshare", "-mrpf", "sh", "-euxc", "mount -t proc none /proc && echo h > /proc/sysrq-trigger"]
			| .process.capabilities.bounding += ["CAP_SETFCAP"]
			| .process.capabilities.permitted += ["CAP_SETFCAP"]'

	runc run --no-pivot test_no_pivot
	[ "$status" -eq 1 ]
	[[ "$output" == *"mount: permission denied"* ]]
}
