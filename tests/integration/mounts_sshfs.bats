#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/echo", "Hello World"]'
}

function teardown() {
	if [ -v DIR ]; then
		# Some distros do not have fusermount installed
		# as a dependency of fuse-sshfs, and good ol' umount works.
		fusermount -u "$DIR" || umount "$DIR"
		unset DIR
	fi

	teardown_bundle
}

function setup_sshfs() {
	# Create a fuse-sshfs mount; skip the test if it's not working.
	local sshfs="sshfs
		-o UserKnownHostsFile=/dev/null
		-o StrictHostKeyChecking=no
		-o PasswordAuthentication=no"

	DIR="$BATS_RUN_TMPDIR/fuse-sshfs"
	mkdir -p "$DIR"

	if ! $sshfs -o "$1" rootless@localhost: "$DIR"; then
		skip "test requires working sshfs mounts"
	fi
}

@test "runc run [implied-rw bind mount of a ro fuse sshfs mount]" {
	setup_sshfs "ro"
	# All of the extra mount flags are needed to force a remount with new flags.
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$DIR"'",
					destination: "/mnt",
					options: ["rprivate", "nosuid", "nodev", "rbind", "sync"]
				}]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [explicit-rw bind mount of a ro fuse sshfs mount]" {
	setup_sshfs "ro"
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$DIR"'",
					destination: "/mnt",
					options: ["rw", "rprivate", "nosuid", "nodev", "rbind"]
				}]'

	runc run test_busybox
	# The above will fail because we explicitly requested a mount with a
	# MNT_LOCKED mount option cleared (when the source mount has those mounts
	# enabled), namely MS_RDONLY.
	[ "$status" -ne 0 ]
	[[ "$output" == *"runc run failed: unable to start container process: error during container init: error mounting"*"operation not permitted"* ]]
}

@test "runc run [ro bind mount of a nodev,nosuid,noexec,noatime fuse sshfs mount]" {
	setup_sshfs "nodev,nosuid,noexec,noatime"
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$DIR"'",
					destination: "/mnt",
					options: ["rbind", "ro"]
				}]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [dev,exec,suid,atime bind mount of a nodev,nosuid,noexec,noatime fuse sshfs mount]" {
	requires rootless

	setup_sshfs "nodev,nosuid,noexec,noatime"
	# The "sync" option is used to trigger a remount with the below options.
	# It serves no further purpose. Otherwise only a bind mount without
	# applying the below options will be done.
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$DIR"'",
					destination: "/mnt",
					options: ["dev", "suid", "exec", "atime", "rprivate", "rbind", "sync"]
				}]'

	runc run test_busybox
	# The above will fail because we explicitly requested a mount with several
	# MNT_LOCKED mount options cleared (when the source mount has those mounts
	# enabled).
	[ "$status" -ne 0 ]
	[[ "$output" == *"runc run failed: unable to start container process: error during container init: error mounting"*"operation not permitted"* ]]
}

@test "runc run [ro,noexec bind mount of a nosuid,noatime fuse sshfs mount]" {
	requires rootless

	setup_sshfs "nosuid,noatime"
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$DIR"'",
					destination: "/mnt",
					options: ["rbind", "ro", "exec"]
				}]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}
