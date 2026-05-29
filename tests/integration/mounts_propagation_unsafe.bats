#!/usr/bin/env bats

load helpers

function require_mount_namespace_tools() {
	command -v unshare >/dev/null || skip "test requires unshare"
	command -v nsenter >/dev/null || skip "test requires nsenter"
}

function in_mount_namespace() {
	local cwd
	cwd="$(pwd)"
	nsenter --mount="$ISOLATED_MNTNS" -- sh -c "cd \"\$1\" && shift && exec \"\$@\"" sh "$cwd" "$@"
}

function setup_isolated_mount_namespace() {
	ISOLATED_MNTNS_DIR="$(mktemp -d "$BATS_RUN_TMPDIR/mntns.XXXXXX")"
	mount --bind "$ISOLATED_MNTNS_DIR" "$ISOLATED_MNTNS_DIR"
	mount --make-private "$ISOLATED_MNTNS_DIR"

	ISOLATED_MNTNS="$ISOLATED_MNTNS_DIR/testns"
	touch "$ISOLATED_MNTNS"
	if ! unshare --mount="$ISOLATED_MNTNS" mount --make-rprivate /; then
		rm -f "$ISOLATED_MNTNS"
		umount "$ISOLATED_MNTNS_DIR" 2>/dev/null || true
		rmdir "$ISOLATED_MNTNS_DIR" 2>/dev/null || true
		fail "failed to bind isolated mount namespace"
	fi
}

function teardown_isolated_mount_namespace() {
	if [ -n "${ISOLATED_MNTNS_DIR:-}" ]; then
		umount -l "$ISOLATED_MNTNS_DIR" 2>/dev/null || true
		rmdir "$ISOLATED_MNTNS_DIR" 2>/dev/null || true
	fi
}

function __runc_in_mount_namespace() {
	setup_runc_cmdline
	in_mount_namespace "${RUNC_CMDLINE[@]}" "$@"
}

function make_rootfs_shared() {
	in_mount_namespace mount --make-rshared /
}

function runc_in_mount_namespace() {
	CMDNAME="$(basename "$RUNC")" sane_run __runc_in_mount_namespace "$@"
}

function setup() {
	requires root
	require_mount_namespace_tools

	setup_isolated_mount_namespace
	make_rootfs_shared
	setup_debian
}

function teardown() {
	teardown_bundle
	teardown_isolated_mount_namespace
}

@test "runc run [rootfsPropagation slave]" {
	# make sure the rootfs mount is slave before running the test
	update_config ' .linux.rootfsPropagation = "slave" '

	update_config ' .process.args = ["findmnt", "--noheadings", "-o", "PROPAGATION", "/"] '

	runc_in_mount_namespace run test_slave_rootfs
	[ "$status" -eq 0 ]
	[ "$output" = "private,slave" ]
}
