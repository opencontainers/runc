#!/usr/bin/env bats

load helpers

function rootfs_propagation() {
	findmnt --noheadings --output PROPAGATION / | tr -d '[:space:]'
}

function make_rootfs_shared() {
	mount --make-rshared /
}

function restore_rootfs_propagation() {
	case "$ORIGINAL_ROOTFS_PROPAGATION" in
	shared)
		mount --make-rshared /
		;;
	slave | private,slave)
		mount --make-rslave /
		;;
	private)
		mount --make-rprivate /
		;;
	unbindable)
		mount --make-runbindable /
		;;
	*)
		echo "unknown rootfs propagation: $ORIGINAL_ROOTFS_PROPAGATION" >&2
		return 1
		;;
	esac
}

function setup() {
	[ "${BATS_UNSAFE_TEST:-}" != "yes" ] && skip "disabled unless BATS_UNSAFE_TEST=yes"
	requires root

	ORIGINAL_ROOTFS_PROPAGATION="$(rootfs_propagation)"
	make_rootfs_shared
	setup_debian
}

function teardown() {
	if [ -n "${ORIGINAL_ROOTFS_PROPAGATION:-}" ]; then
		restore_rootfs_propagation
	fi
	teardown_bundle
}

@test "runc run [rootfsPropagation slave]" {
	# make sure the rootfs mount is slave before running the test
	update_config ' .linux.rootfsPropagation = "slave" '

	update_config ' .process.args = ["findmnt", "--noheadings", "-o", "PROPAGATION", "/"] '

	runc run test_slave_rootfs
	[ "$status" -eq 0 ]
	[ "$output" = "private,slave" ]
}
