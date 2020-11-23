#!/usr/bin/env bats

load helpers

# This test must be particular with how it's run. The user that runs it must have the privileges
# to chown a directory it creates away from itself, but not have CAP_DAC_OVERRIDE to override
# the fact it was chowned away. However, it must also be able to clean up.
# Thus, this test must be skipped if the UID running the test is root, or the user doesn't have sudo privileges.
function setup() {
	if [ "$EUID" -eq 0 ]; then
		skip "This test must be run as a non-root user"
	fi
	if ! sudo -n -v; then
		skip "This test must be run as a user with sudo privileges"
	fi

	teardown_busybox
	setup_busybox

	SOURCE=$(mktemp -d "$BATS_TMPDIR/cwd.XXXXXX")
}

function teardown() {
	sudo -n rm -rf "$SOURCE"
	teardown_busybox
}

# Verify a cwd owned by the container user can be chdir'd to,
# even if runc doesn't have the privilege to do so.
@test "runc create sets up user before chdir to cwd" {
	USER=10000001
	sudo -n chown $USER:$USER "$SOURCE"
	ls -ld "$SOURCE" >&3
	# shellcheck disable=SC2016
	update_config ' .mounts += [{"source": "'"$SOURCE"'", "destination": "'"$SOURCE"'", "options": ["bind"]}]
				| .process.user.uid = $USER
				| .process.user.gid = $USER
				| .process.cwd = "'"$SOURCE"'"
				| .process.args |= ["ls", "'"$SOURCE"'"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${output}" != *"Permission denied"* ]]
}
