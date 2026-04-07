#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# Test case for https://github.com/opencontainers/runc/pull/2086
@test "runc exec --user with no access to cwd" {
	requires root

	chown 42 rootfs/root
	chmod 700 rootfs/root

	update_config '	  .process.cwd = "/root"
			| .process.user.uid = 42
			| .process.args |= ["sleep", "1h"]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --user 0 test_busybox true
	[ "$status" -eq 0 ]
}

# Verify a cwd owned by the container user can be chdir'd to,
# even if runc doesn't have the privilege to do so.
@test "runc create sets up user before chdir to cwd if needed" {
	requires rootless rootless_idmap

	# Some setup for this test (ROOTLESS_AUX_DIR and ROOTLESS_AUX_UID) is done
	# by rootless.sh. Check that setup is done...
	if [[ ! -v ROOTLESS_AUX_UID || ! -v ROOTLESS_AUX_DIR || ! -d "$ROOTLESS_AUX_DIR" ]]; then
		skip "bad/unset ROOTLESS_AUX_DIR/ROOTLESS_AUX_UID"
	fi
	# ... and is correct, i.e. the current user
	# does not have permission to access ROOTLESS_AUX_DIR.
	if ls -l "$ROOTLESS_AUX_DIR" 2>/dev/null; then
		skip "bad ROOTLESS_AUX_DIR permissions"
	fi

	update_config '   .mounts += [{
				source: "'"$ROOTLESS_AUX_DIR"'",
				destination: "'"$ROOTLESS_AUX_DIR"'",
				options: ["bind"]
			    }]
			| .process.user.uid = '"$ROOTLESS_AUX_UID"'
			| .process.cwd = "'"$ROOTLESS_AUX_DIR"'"
			| .process.args |= ["ls", "'"$ROOTLESS_AUX_DIR"'"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Verify a cwd not owned by the container user can be chdir'd to,
# if runc does have the privilege to do so.
@test "runc create can chdir if runc has access" {
	requires root

	mkdir -p rootfs/home/nonroot
	chmod 700 rootfs/home/nonroot

	update_config '	  .process.cwd = "/root"
			| .process.user.uid = 42
			| .process.args |= ["ls", "/tmp"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}
