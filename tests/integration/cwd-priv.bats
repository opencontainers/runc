#!/usr/bin/env bats

load helpers

function setup() {
	requires rootless rootless_idmap

	# Some setup for this test (AUX_DIR and AUX_UID) is done
	# by rootless.sh. Check that setup is done...
	if [[ ! -d "$AUX_DIR" || -z "$AUX_UID" ]]; then
		skip "bad/unset AUX_DIR/AUX_UID"
	fi
	# ... and is correct, i.e. the current user
	# does not have permission to access AUX_DIR.
	if ls -l "$AUX_DIR" 2>/dev/null; then
		skip "bad AUX_DIR permissions"
	fi
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

# Verify a cwd owned by the container user can be chdir'd to,
# even if runc doesn't have the privilege to do so.
#
@test "runc create sets up user before chdir to cwd" {
	update_config '   .mounts += [{
				source: "'"$AUX_DIR"'",
				destination: "'"$AUX_DIR"'",
				options: ["bind"]
			    }]
			| .process.user.uid = '"$AUX_UID"'
			| .process.cwd = "'"$AUX_DIR"'"
			| .process.args |= ["ls", "'"$AUX_DIR"'"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc exec --user with no access to cwd" {
	update_config '   .mounts += [{
				source: "'"$AUX_DIR"'",
				destination: "'"$AUX_DIR"'",
				options: ["bind"]
			    }]
			| .process.user.uid = '"$AUX_UID"'
			| .process.cwd = "'"$AUX_DIR"'"
			| .process.args |= ["sleep", "1h"]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --user 0 test_busybox true
	[ "$status" -eq 0 ]
}
