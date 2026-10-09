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

	run -1 runc run --no-pivot test_no_pivot
	assert_output --partial "mount: permission denied"
}

@test "runc run --no-pivot [host mount ns] must fail" {
	# Unsafe because, if the bug is present, host's /proc and /sys
	# are unmounted, breaking the host.
	requires root unsafe

	update_config '	  .linux.namespaces -= [{"type": "mount"}]
			| .linux.maskedPaths = []
			| .linux.readonlyPaths = []
			| .root.readonly = false'

	run -1 runc run --no-pivot test_no_pivot
	assert_output --partial "unable to use no-pivot without a private MNT namespace"
}
