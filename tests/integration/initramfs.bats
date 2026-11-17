#!/usr/bin/env bats

# Tests for running runc from initramfs, using a qemu VM with the host kernel
# and a custom initramfs (built from busybox image, runc, and a bundle).

load helpers

function setup() {
	setup_initramfs
	update_config '.process.args = ["sh", "-c", "test -e /CONTAINER && echo CONTAINER-OK"]'
}

function teardown() {
	teardown_initramfs
}

# Commands to check host mounts are not changed by runc.
SAVE_MOUNTS='cat /proc/self/mountinfo >/tmp/mounts'
CHECK_MOUNTS='cat /proc/self/mountinfo | diff /tmp/mounts - && echo HOST-MOUNTS-OK'

@test "runc run [initramfs]" {
	run_initramfs "$SAVE_MOUNTS" "runc run ctr" "$CHECK_MOUNTS"
	assert_output --partial "CONTAINER-OK"
	assert_output --partial "HOST-MOUNTS-OK"
}

@test "runc run [initramfs, rshared]" {
	update_config '.linux.rootfsPropagation = "rshared"'
	run_initramfs "mount --make-rshared /" "$SAVE_MOUNTS" "runc run ctr" "$CHECK_MOUNTS"
	assert_output --partial "CONTAINER-OK"
	assert_output --partial "HOST-MOUNTS-OK"
}

@test "runc run --no-pivot [initramfs]" {
	run_initramfs "runc run --no-pivot ctr"
	assert_output --partial "CONTAINER-OK"
	assert_output --partial "--no-pivot is deprecated and ignored"
}

# Make the container try to mount fresh procfs and sysfs from a new user
# namespace, which must fail as long as there are no fully visible procfs
# and sysfs mounts in its mount namespace.
function set_fresh_mounts_args() {
	update_config '	  .process.args = ["unshare", "-mrpnf", "sh", "-c", "mount -t proc none /proc; mount -t sysfs none /sys; echo DONE"]
			| .process.capabilities.bounding += ["CAP_SETFCAP"]
			| .process.capabilities.permitted += ["CAP_SETFCAP"]'
}

@test "runc run [initramfs] must not expose bare /proc and /sys" {
	set_fresh_mounts_args
	run_initramfs "runc run ctr"
	assert_output --partial "DONE"
	assert_output --regexp "mount: permission denied.*mount: permission denied"
}

@test "runc run [initramfs, stacked /proc and /sys] must not expose bare /proc and /sys" {
	set_fresh_mounts_args
	run_initramfs "mount -t proc proc /proc" "mount --rbind /sys /sys" "runc run ctr"
	assert_output --partial "DONE"
	assert_output --regexp "mount: permission denied.*mount: permission denied"
}

@test "runc run [initramfs, userns]" {
	update_config '	  .linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings = [{"containerID": 0, "hostID": 0, "size": 65536}]
			| .linux.gidMappings = [{"containerID": 0, "hostID": 0, "size": 65536}]'
	run_initramfs "runc run ctr || echo RUNC-FAILED"
	if is_kernel_gte 7.0; then
		assert_output --partial "CONTAINER-OK"
	else
		# Mounts are locked, and can't be detached from the initramfs.
		assert_output --partial "RUNC-FAILED"
		assert_output --partial "requires Linux >= 7.0"
	fi
}
