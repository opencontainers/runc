#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
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

function sshfs_has_flag() {
	if [ -v DIR ]; then
		awk '$2 == "'"$DIR"'" { print $4 }' </proc/self/mounts | grep -E "\b$1\b"
		return "$?"
	fi
}

function setup_sshfs() {
	# Create a fuse-sshfs mount (or, failing that, a tmpfs mount).
	local sshfs="sshfs
		-o UserKnownHostsFile=/dev/null
		-o StrictHostKeyChecking=no
		-o PasswordAuthentication=no"

	if ! [ -v DIR ]; then
		DIR="$BATS_RUN_TMPDIR/fuse-sshfs"
		mkdir -p "$DIR"
		# Make sure we clear all superblock flags to make sure bind-mounts can
		# unset these flags.
		if ! $sshfs -o rw,suid,dev,exec,atime rootless@localhost: "$DIR"; then
			# fallback to tmpfs if running in without sshfs
			mount -t tmpfs -o rw,suid,dev,exec,diratime,strictatime tmpfs "$DIR"
		fi
	fi
	# Reset atime flags. "diratime" is quite a strange flag, so we need to make
	# sure it's cleared before we apply the requested flags.
	mount --bind -o remount,diratime,strictatime "$DIR"
	# We need to set the mount flags separately on the mount because some mount
	# flags (such as "ro") are set on the superblock if you do them in the
	# initial mount, which means that they cannot be cleared by bind-mounts.
	#
	# This also lets us reconfigure the per-mount settings on each call.
	mount --bind -o "remount,$1" "$DIR"
	echo "configured $DIR with mount --bind -o remount,$1" >&2
	awk '$2 == "'"$DIR"'"' </proc/self/mounts >&2
}

function setup_sshfs_bind_flags() {
	host_flags="$1" # ro,nodev,nosuid
	bind_flags="$2" # ro,nosuid,bind

	setup_sshfs "$host_flags"

	cat >"rootfs/find-tmp.awk" <<-'EOF'
		#!/bin/awk -f
		$2 == "/mnt" { print $4 }
	EOF
	chmod +x "rootfs/find-tmp.awk"

	update_config '.process.args = ["sh", "-c", "/find-tmp.awk </proc/self/mounts"]'
	update_config '.mounts = (.mounts | map(select(.destination != "/mnt"))) + [{
			"source": "'"$DIR"'",
			"destination": "/mnt",
			"type": "bind",
			"options": '"$(jq -cRM 'split(",")' <<<"$bind_flags")"'
		}]'
}

function pass_sshfs_bind_flags() {
	setup_sshfs_bind_flags "$@"

	runc run test_busybox
	[ "$status" -eq 0 ]
	mnt_flags="$output"
}

function fail_sshfs_bind_flags() {
	setup_sshfs_bind_flags "$@"

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"runc run failed: unable to start container process: error during container init: error mounting"*"operation not permitted"* ]]
}

@test "runc run [mount(8)-like behaviour: --bind with no options]" {
	requires root

	pass_sshfs_bind_flags "ro,noexec,nosymfollow,nodiratime" "bind"
	# If no flags were specified alongside bind, we keep all existing flags.
	# Unspecified flags must be cleared (rw default).
	run -0 grep -wq ro <<<"$mnt_flags"
	run ! grep -wq rw <<<"$mnt_flags"
	run -0 grep -wq noexec <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	# On old systems, mount doesn't know about nosymfollow, which turns the
	# flag into a data argument (which is ignored by MS_REMOUNT).
	if sshfs_has_flag nosymfollow; then run -0 grep -wq nosymfollow <<<"$mnt_flags"; fi

	# Now try with a user namespace. The results should be the same as above.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	pass_sshfs_bind_flags "ro,noexec,nosymfollow,nodiratime" "bind"
	# If no flags were specified alongside bind, we keep all existing flags.
	# Unspecified flags must be cleared (rw default).
	run -0 grep -wq ro <<<"$mnt_flags"
	run ! grep -wq rw <<<"$mnt_flags"
	run -0 grep -wq noexec <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	# On old systems, mount doesn't know about nosymfollow, which turns the
	# flag into a data argument (which is ignored by MS_REMOUNT).
	if sshfs_has_flag nosymfollow; then run -0 grep -wq nosymfollow <<<"$mnt_flags"; fi
}

# This behaviour does not match mount(8), but is preferable to the alternative.
# See <https://github.com/util-linux/util-linux/issues/2433>.
@test "runc run [mount(8)-unlike behaviour: --bind with clearing flag]" {
	requires root

	pass_sshfs_bind_flags "ro,noexec,nosymfollow,nodiratime" "bind,dev"
	# Unspecified flags must be cleared as well.
	run ! grep -wq ro <<<"$mnt_flags"
	run -0 grep -wq rw <<<"$mnt_flags"
	run ! grep -wq noexec <<<"$mnt_flags"
	run ! grep -wq nosymfollow <<<"$mnt_flags"
	# FIXME FIXME: As with mount(8), trying to clear an atime flag the "naive"
	# way will be ignored!
	run -0 grep -wq nodiratime <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	pass_sshfs_bind_flags "ro,noexec,nosymfollow,nodiratime" "bind,dev"
	# Lockable flags must be kept, because we didn't request them explicitly.
	run -0 grep -wq ro <<<"$mnt_flags"
	run ! grep -wq rw <<<"$mnt_flags"
	run -0 grep -wq noexec <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	# nosymfollow is not lockable, so it must be cleared.
	run ! grep -wq nosymfollow <<<"$mnt_flags"
}

@test "runc run [implied-rw bind mount of a ro fuse sshfs mount]" {
	requires root

	pass_sshfs_bind_flags "ro" "bind,nosuid,nodev,rprivate"
	# Unspecified flags must be cleared (rw default).
	run ! grep -wq ro <<<"$mnt_flags"
	run -0 grep -wq rw <<<"$mnt_flags"
	# The new flags must be applied.
	run -0 grep -wq nosuid <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"

	# Now try with a user namespace. The results should be the same as above.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	pass_sshfs_bind_flags "ro" "bind,nosuid,nodev,rprivate"
	# "ro" must still be set (inherited).
	run -0 grep -wq ro <<<"$mnt_flags"
	# The new flags must be applied.
	run -0 grep -wq nosuid <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"
}

@test "runc run [explicit-rw bind mount of a ro fuse sshfs mount]" {
	requires root

	# Try to overwrite MS_RDONLY. As we are running in a userns-less container,
	# we can overwrite MNT_LOCKED flags.
	pass_sshfs_bind_flags "ro" "bind,rw,nosuid,nodev,rprivate"
	# "ro" must be cleared and replaced with "rw".
	run ! grep -wq ro <<<"$mnt_flags"
	run -0 grep -wq rw <<<"$mnt_flags"
	# The new flags must be applied.
	run -0 grep -wq nosuid <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	# This must fail because we explicitly requested a mount with a MNT_LOCKED
	# mount option cleared (when the source mount has those mounts enabled),
	# namely MS_RDONLY.
	fail_sshfs_bind_flags "ro" "bind,rw,nosuid,nodev,rprivate"
}

@test "runc run [dev,exec,suid,atime bind mount of a nodev,nosuid,noexec,noatime fuse sshfs mount]" {
	requires root

	# When running without userns, overwriting host flags should work.
	pass_sshfs_bind_flags "nosuid,nodev,noexec,noatime" "bind,dev,suid,exec,atime"
	# Unspecified flags must be cleared (rw default).
	run ! grep -wq ro <<<"$mnt_flags"
	run -0 grep -wq rw <<<"$mnt_flags"
	# Check that the flags were actually cleared by the mount.
	run ! grep -wq nosuid <<<"$mnt_flags"
	run ! grep -wq nodev <<<"$mnt_flags"
	run ! grep -wq noexec <<<"$mnt_flags"
	# FIXME FIXME: As with mount(8), trying to clear an atime flag the "naive"
	# way will be ignored!
	run -0 grep -wq noatime <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	# This must fail because we explicitly requested a mount with MNT_LOCKED
	# mount options cleared (when the source mount has those mounts enabled).
	fail_sshfs_bind_flags "nodev,nosuid,nosuid,noatime" "bind,dev,suid,exec,atime"
}

# Test to ensure we don't regress bind-mounting /etc/resolv.conf with
# containerd <https://github.com/containerd/containerd/pull/8309>.
@test "runc run [ro bind mount of a nodev,nosuid,noexec fuse sshfs mount]" {
	requires root

	# Setting flags that are not locked should work.
	pass_sshfs_bind_flags "rw,nodev,nosuid,nodev,noexec,noatime" "bind,ro"
	# The flagset should be the union of the two.
	run -0 grep -wq ro <<<"$mnt_flags"
	# Unspecified flags must be cleared.
	run ! grep -wq nosuid <<<"$mnt_flags"
	run ! grep -wq nodev <<<"$mnt_flags"
	run ! grep -wq noexec <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	# Setting flags that are not locked should work.
	pass_sshfs_bind_flags "rw,nodev,nosuid,nodev,noexec,noatime" "bind,ro"
	# The flagset should be the union of the two.
	run -0 grep -wq ro <<<"$mnt_flags"
	# (Unspecified MNT_LOCKED flags are inherited.)
	run -0 grep -wq nosuid <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"
	run -0 grep -wq noexec <<<"$mnt_flags"
}

@test "runc run [ro,symfollow bind mount of a rw,nodev,nosymfollow fuse sshfs mount]" {
	requires root

	pass_sshfs_bind_flags "rw,nodev,nosymfollow" "bind,ro,symfollow"
	# Must switch to ro.
	run -0 grep -wq ro <<<"$mnt_flags"
	run ! grep -wq rw <<<"$mnt_flags"
	# Unspecified flags must be cleared.
	run ! grep -wq nodev <<<"$mnt_flags"
	# nosymfollow must also be cleared.
	run ! grep -wq nosymfollow <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	# Unsetting flags that are not lockable should work.
	pass_sshfs_bind_flags "rw,nodev,nosymfollow" "bind,ro,symfollow"
	# The flagset should be the union of the two.
	run -0 grep -wq ro <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"
	# nosymfollow is not lockable, so it must be cleared.
	run ! grep -wq nosymfollow <<<"$mnt_flags"

	# Implied unsetting of non-lockable flags should also work.
	pass_sshfs_bind_flags "rw,nodev,nosymfollow" "bind,rw"
	# The flagset should be the union of the two.
	run -0 grep -wq rw <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"
	# nosymfollow is not lockable, so it must be cleared.
	run ! grep -wq nosymfollow <<<"$mnt_flags"
}

@test "runc run [ro,noexec bind mount of a nosuid,noatime fuse sshfs mount]" {
	requires root

	# Setting flags that are not locked should work.
	pass_sshfs_bind_flags "nodev,nosuid,noatime" "bind,ro,exec"
	# The flagset must match the requested set.
	run -0 grep -wq ro <<<"$mnt_flags"
	run ! grep -wq noexec <<<"$mnt_flags"
	# Unspecified flags must be cleared.
	run ! grep -wq nosuid <<<"$mnt_flags"
	run ! grep -wq nodev <<<"$mnt_flags"
	# FIXME: As with mount(8), runc keeps the old atime setting by default.
	run -0 grep -wq noatime <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	# Setting flags that are not locked should work.
	pass_sshfs_bind_flags "nodev,nosuid,noatime" "bind,ro,exec"
	# The flagset should be the union of the two.
	run -0 grep -wq ro <<<"$mnt_flags"
	run ! grep -wq noexec <<<"$mnt_flags"
	# (Unspecified MNT_LOCKED flags are inherited.)
	run -0 grep -wq nosuid <<<"$mnt_flags"
	run -0 grep -wq nodev <<<"$mnt_flags"
	run -0 grep -wq noatime <<<"$mnt_flags"
}

@test "runc run [bind mount {no,rel,strict}atime semantics]" {
	requires root

	function is_strictatime() {
		# There is no "strictatime" in /proc/self/mounts.
		run ! grep -wq noatime <<<"${1:-$mnt_flags}"
		run ! grep -wq relatime <<<"${1:-$mnt_flags}"
		run ! grep -wq nodiratime <<<"${1:-$mnt_flags}"
	}

	# FIXME: As with mount(8), runc keeps the old atime setting by default.
	pass_sshfs_bind_flags "noatime" "bind"
	run -0 grep -wq noatime <<<"$mnt_flags"
	run ! grep -wq relatime <<<"$mnt_flags"

	# FIXME: As with mount(8), runc keeps the old atime setting by default.
	pass_sshfs_bind_flags "noatime" "bind,norelatime"
	run -0 grep -wq noatime <<<"$mnt_flags"
	run ! grep -wq relatime <<<"$mnt_flags"

	# FIXME FIXME: As with mount(8), trying to clear an atime flag the "naive"
	# way will be ignored!
	pass_sshfs_bind_flags "noatime" "bind,atime"
	run -0 grep -wq noatime <<<"$mnt_flags"
	run ! grep -wq relatime <<<"$mnt_flags"

	# ... but explicitly setting a different flag works.
	pass_sshfs_bind_flags "noatime" "bind,relatime"
	run ! grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq relatime <<<"$mnt_flags"

	# Setting a flag that mount(8) would combine should result in only the
	# requested flag being set.
	pass_sshfs_bind_flags "noatime" "bind,nodiratime"
	run ! grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	# MS_DIRATIME implies MS_RELATIME by default.
	run -0 grep -wq relatime <<<"$mnt_flags"

	# Clearing flags that mount(8) would not clear works.
	pass_sshfs_bind_flags "nodiratime" "bind,strictatime"
	is_strictatime "$mnt_flags"

	# nodiratime is a little weird -- it implies relatime unless you set
	# another option (noatime or strictatime). But, runc also has norelatime --
	# so nodiratime,norelatime should _probably_ result in the same thing as
	# nodiratime,strictatime.
	pass_sshfs_bind_flags "noatime" "bind,nodiratime,strictatime"
	run ! grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	run ! grep -wq relatime <<<"$mnt_flags"
	# FIXME FIXME: relatime should not be set in this case.
	pass_sshfs_bind_flags "noatime" "bind,nodiratime,norelatime"
	run ! grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	run -0 grep -wq relatime <<<"$mnt_flags"

	# Now try with a user namespace.
	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '

	# Requesting a mount without specifying any preference for atime works, and
	# inherits the original flags.

	pass_sshfs_bind_flags "strictatime" "bind"
	is_strictatime "$mnt_flags"

	pass_sshfs_bind_flags "relatime" "bind"
	run -0 grep -wq relatime <<<"$mnt_flags"

	pass_sshfs_bind_flags "nodiratime" "bind"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	# MS_DIRATIME implies MS_RELATIME by default.
	run -0 grep -wq relatime <<<"$mnt_flags"

	pass_sshfs_bind_flags "noatime,nodiratime" "bind"
	run -0 grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"

	# An unrelated clear flag has no effect.
	pass_sshfs_bind_flags "noatime,nodiratime" "bind,norelatime"
	run -0 grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"

	# Attempting to change most *atime flags will fail with user namespaces
	# because *atime flags are all MNT_LOCKED.
	fail_sshfs_bind_flags "nodiratime" "bind,strictatime"
	fail_sshfs_bind_flags "relatime" "bind,strictatime"
	fail_sshfs_bind_flags "noatime" "bind,strictatime"
	fail_sshfs_bind_flags "nodiratime" "bind,noatime"
	fail_sshfs_bind_flags "relatime" "bind,noatime"
	fail_sshfs_bind_flags "relatime" "bind,nodiratime"
	# Make sure strictatime sources are correctly handled by runc (the kernel
	# ignores some other mount flags when passing MS_STRICTATIME). See
	# remount() in rootfs_linux.go for details.
	fail_sshfs_bind_flags "strictatime" "bind,relatime"
	fail_sshfs_bind_flags "strictatime" "bind,noatime"
	fail_sshfs_bind_flags "strictatime" "bind,nodiratime"
	# Make sure that runc correctly handles the MS_NOATIME|MS_RELATIME kernel
	# bug. See remount() in rootfs_linux.go for more details.
	fail_sshfs_bind_flags "noatime" "bind,relatime"

	# Attempting to bind-mount a mount with a request to clear the atime
	# setting that would normally inherited must not work.
	# FIXME FIXME: All of these cases should fail.
	pass_sshfs_bind_flags "strictatime" "bind,nostrictatime"
	is_strictatime "$mnt_flags"
	pass_sshfs_bind_flags "nodiratime" "bind,diratime"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	pass_sshfs_bind_flags "nodiratime" "bind,norelatime" # MS_DIRATIME implies MS_RELATIME
	run -0 grep -wq nodiratime <<<"$mnt_flags"
	pass_sshfs_bind_flags "relatime" "bind,norelatime"
	run -0 grep -wq relatime <<<"$mnt_flags"
	pass_sshfs_bind_flags "noatime" "bind,atime"
	run -0 grep -wq noatime <<<"$mnt_flags"
	pass_sshfs_bind_flags "noatime,nodiratime" "bind,atime"
	run -0 grep -wq noatime <<<"$mnt_flags"
	run -0 grep -wq nodiratime <<<"$mnt_flags"
}
