#!/usr/bin/env bats

load helpers

function setup() {
	# Create a ro fuse-sshfs mount; skip the test if it's not working.
	local sshfs="sshfs
		-o UserKnownHostsFile=/dev/null
		-o StrictHostKeyChecking=no
		-o PasswordAuthentication=no"

	DIR="$BATS_RUN_TMPDIR/fuse-sshfs"
	mkdir -p "$DIR"

	if ! $sshfs -o ro rootless@localhost: "$DIR"; then
		skip "test requires working sshfs mounts"
	fi

	setup_hello
}

function teardown() {
	# New distros (Fedora 35) do not have fusermount installed
	# as a dependency of fuse-sshfs, and good ol' umount works.
	fusermount -u "$DIR" || umount "$DIR"

	teardown_bundle
}

@test "runc run [rw bind mount of a ro fuse sshfs mount]" {
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$DIR"'",
					destination: "/mnt",
					options: ["rw", "rprivate", "nosuid", "nodev", "rbind"]
				}]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}
