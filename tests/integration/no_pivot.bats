#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

@test "runc run --no-pivot must not expose bare /proc" {
	requires root

	update_config '.process.args |= ["unshare", "-mrpf", "sh", "-euxc", "mount -t proc none /proc && echo h > /proc/sysrq-trigger"]'

	runc run --no-pivot test_no_pivot
	[ "$status" -eq 1 ]
	[[ "$output" == *"mount: permission denied"* ]]
}
