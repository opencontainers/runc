#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	if [ -v ROOT ] && mountpoint -q "$ROOT/bundle/rootfs"; then
		umount -R --lazy "$ROOT/bundle/rootfs"
	fi
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

# https://github.com/opencontainers/runc/issues/1961
@test "runc run --no-pivot [rootfsPropagation rshared] must not leak rootfs mount" {
	# Unsafe because, if the bug is present, the leaked mount on top
	# of the host's "/" can not be removed, breaking the host.
	requires root unsafe

	update_config '	  .linux.rootfsPropagation = "rshared"
			| .process.args = ["true"]
			| .process.terminal = false'

	# The issue is only reproducible if rootfs is a mount.
	mount --bind rootfs rootfs

	before=$(awk '$5 == "/"' /proc/self/mountinfo | wc -l)
	run -0 runc run --no-pivot test_no_pivot
	after=$(awk '$5 == "/"' /proc/self/mountinfo | wc -l)
	[ "$before" -eq "$after" ]
}
