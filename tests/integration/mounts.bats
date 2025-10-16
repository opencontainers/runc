#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# https://github.com/opencontainers/runc/security/advisories/GHSA-m8cg-xc2p-r3fc
#
# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function test_ro_cgroup_mount() {
	local lines status
	# shellcheck disable=SC2016
	update_config '.process.args |= ["sh", "-euc", "for f in `grep /sys/fs/cgroup /proc/mounts | awk \"{print \\\\$2}\"| uniq`; do test -e $f && grep -w $f /proc/mounts | tail -n1; done"]'
	runc -0 run test_busybox
	[ "${#lines[@]}" -ne 0 ]
	for line in "${lines[@]}"; do [[ "${line}" == *'ro,'* ]]; done
}

# Parse an "optstring" of the form foo,bar into $is_foo and $is_bar variables.
# Usage: parse_optstring foo,bar foo bar baz
function parse_optstring() {
	optstring="$1"
	shift

	for flag in "$@"; do
		is_set=
		if grep -wq "$flag" <<<"$optstring"; then
			is_set=1
		fi
		eval "is_$flag=$is_set"
	done
}

function config_add_bind_mount() {
	src="$1"
	dst="$2"
	parse_optstring "${3:-}" rbind idmap

	bindtype=bind
	if [ -n "$is_rbind" ]; then
		bindtype=rbind
	fi

	mappings=""
	if [ -n "$is_idmap" ]; then
		mappings='
			"uidMappings": [{"containerID": 0, "hostID": 100000, "size": 65536}],
			"gidMappings": [{"containerID": 0, "hostID": 100000, "size": 65536}],
		'
	fi

	update_config '.mounts += [{
		"source": "'"$src"'",
		"destination": "'"$dst"'",
		"type": "bind",
		'"$mappings"'
		"options": [ "'"$bindtype"'", "rprivate" ]
	}]'
}

# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function test_mount_order() {
	parse_optstring "${1:-}" userns idmap

	if [ -n "$is_userns" ]; then
		requires root

		update_config '.linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
			| .linux.gidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]'
		remap_rootfs
	fi

	ctr_src_opts="rbind"
	if [ -n "$is_idmap" ]; then
		requires root
		requires_kernel 5.12
		requires_idmap_fs .

		ctr_src_opts+=",idmap"
	fi

	mkdir -p rootfs/{mnt,final}
	# Create a set of directories we can create a mount tree with.
	for subdir in a/x b/y c/z; do
		dir="bind-src/$subdir"
		mkdir -p "$dir"
		echo "$subdir" >"$dir/file"
		# Add a symlink to make sure
		topdir="$(dirname "$subdir")"
		ln -s "$topdir" "bind-src/sym-$topdir"
	done
	# In userns tests, make sure that the source directory cannot be accessed,
	# to make sure we're exercising the bind-mount source fd logic.
	chmod o-rwx bind-src

	rootfs="$(pwd)/rootfs"
	rm -rf rootfs/mnt
	mkdir rootfs/mnt

	# Create a bind-mount tree.
	config_add_bind_mount "$PWD/bind-src/a" "/mnt"
	config_add_bind_mount "$PWD/bind-src/sym-b" "/mnt/x"
	config_add_bind_mount "$PWD/bind-src/c" "/mnt/x/y"
	config_add_bind_mount "$PWD/bind-src/sym-a" "/mnt/x/y/z"
	# Create a recursive bind-mount that uses part of the current tree in the
	# container.
	config_add_bind_mount "$rootfs/mnt/x" "$rootfs/mnt/x/y/z/x" "$ctr_src_opts"
	config_add_bind_mount "$rootfs/mnt/x/y" "$rootfs/mnt/x/y/z" "$ctr_src_opts"
	# Finally, bind-mount the whole thing on top of /final.
	config_add_bind_mount "$rootfs/mnt" "$rootfs/final" "$ctr_src_opts"

	# Check that the entire tree was copied and the mounts were done in the
	# expected order.
	update_config '.process.args = ["cat", "/final/x/y/z/z/x/y/z/x/file"]'
	runc -0 run test_busybox
	[[ "$output" == *"a/x"* ]] # the final "file" was from a/x.
}

# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
test_mount_target() {
	src="$1"
	dst="$2"
	real_dst="${3:-$dst}"

	echo "== $src -> $dst (=> $real_dst) =="

	old_config="$(mktemp ./config.json.bak.XXXXXX)"
	cp ./config.json "$old_config"

	update_config '.mounts += [{
		source: "'"$src"'",
		destination: "'"$dst"'",
		options: ["bind"]
	}]'

	# Make sure the target path is at the right spot and is actually a
	# bind-mount of the correct inode.
	update_config '.process.args = ["stat", "-c", "%n %d:%i", "--", "'"$real_dst"'"]'
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == "$real_dst $(stat -c "%d:%i" -- "$src")" ]]

	# Make sure there is a mount entry for the target path.
	# shellcheck disable=SC2016
	update_config '.process.args = ["awk", "-F", "PATH='"$real_dst"'", "$2 == PATH", "/proc/self/mounts"]'
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"$real_dst"* ]]

	# Switch back the old config so this function can be called multiple times.
	mv "$old_config" "./config.json"
}

# https://github.com/opencontainers/runc/issues/3991
@test "runc run [tmpcopyup]" {
	mkdir -p rootfs/dir1/dir2
	chmod 777 rootfs/dir1/dir2
	update_config '	  .mounts += [{
					source: "tmpfs",
					destination: "/dir1",
					type: "tmpfs",
					options: ["tmpcopyup"]
				}]
			| .process.args |= ["ls", "-ld", "/dir1/dir2"]'

	umask 022
	runc -0 run test_busybox
	[[ "${lines[0]}" == *'drwxrwxrwx'* ]]
}

@test "runc run [bind mount]" {
	update_config '	  .mounts += [{
					source: ".",
					destination: "/tmp/bind",
					options: ["bind"]
				}]
			| .process.args |= ["ls", "/tmp/bind/config.json"]'

	runc -0 run test_busybox
	[[ "${lines[0]}" == *'/tmp/bind/config.json'* ]]
}

# https://github.com/opencontainers/runc/issues/2246
@test "runc run [ro tmpfs mount]" {
	update_config '	  .mounts += [{
					source: "tmpfs",
					destination: "/mnt",
					type: "tmpfs",
					options: ["ro", "nodev", "nosuid", "mode=755"]
				}]
			| .process.args |= ["grep", "^tmpfs /mnt", "/proc/mounts"]'

	runc -0 run test_busybox
	[[ "${lines[0]}" == *'ro,'* ]]
}

# https://github.com/opencontainers/runc/issues/3248
@test "runc run [ro /dev mount]" {
	update_config '   .mounts |= map((select(.destination == "/dev") | .options += ["ro"]) // .)
			| .process.args |= ["grep", "^tmpfs /dev", "/proc/mounts"]'

	runc -0 run test_busybox
	[[ "${lines[0]}" == *'ro,'* ]]
}

# https://github.com/opencontainers/runc/issues/2683
@test "runc run [tmpfs mount with absolute symlink]" {
	# in container, /conf -> /real/conf
	mkdir -p rootfs/real/conf
	ln -s /real/conf rootfs/conf
	update_config '	  .mounts += [{
					type: "tmpfs",
					source: "tmpfs",
					destination: "/conf/stack",
					options: ["ro", "nodev", "nosuid"]
				}]
			| .process.args |= ["true"]'
	runc -0 run test_busybox
}

# CVE-2023-27561 CVE-2019-19921
@test "runc run [/proc is a symlink]" {
	# Make /proc in the container a symlink.
	rm -rf rootfs/proc
	mkdir -p rootfs/bad-proc
	ln -sf /bad-proc rootfs/proc
	# This should fail.
	runc ! run test_busybox
	[[ "$output" == *"must be mounted on ordinary directory"* ]]
}

# https://github.com/opencontainers/runc/issues/4401
@test "runc run [setgid / + mkdirall]" {
	mkdir rootfs/setgid
	chmod '=7755' rootfs/setgid

	update_config '.mounts += [{
		type: "tmpfs",
		source: "tmpfs",
		destination: "/setgid/a/b/c",
		options: ["ro", "nodev", "nosuid"]
	}]'
	update_config '.process.args |= ["true"]'

	runc -0 run test_busybox

	# Verify that the setgid bit is inherited.
	[[ "$(stat -c %a rootfs/setgid)" == 7755 ]]
	[[ "$(stat -c %a rootfs/setgid/a)" == 2755 ]]
	[[ "$(stat -c %a rootfs/setgid/a/b)" == 2755 ]]
	[[ "$(stat -c %a rootfs/setgid/a/b/c)" == 2755 ]]
}

# https://github.com/opencontainers/runc/issues/4971
@test "runc run [tmpfs mount mode= inherit]" {
	mkdir rootfs/tmpfs
	chmod "=0710" rootfs/tmpfs

	update_config '.mounts += [{
		type: "tmpfs",
		source: "tmpfs",
		destination: "/tmpfs",
		options: ["rw", "nodev", "nosuid"]
	}]'
	update_config '.process.args = ["stat", "-c", "%a", "/tmpfs"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == "710" ]]

	update_config '.process.args = ["cat", "/proc/self/mounts"]'
	runc run test_busybox
	[ "$status" -eq 0 ]
	grep -Ex "tmpfs /tmpfs tmpfs [^ ]*\bmode=710\b[^ ]* .*" <<<"$output"
}

# https://github.com/opencontainers/runc/issues/4971
@test "runc run [tmpfs mount explicit mode=]" {
	mkdir rootfs/tmpfs
	chmod "=0710" rootfs/tmpfs

	update_config '.mounts += [{
		type: "tmpfs",
		source: "tmpfs",
		destination: "/tmpfs",
		options: ["rw", "nodev", "nosuid", "mode=1500"]
	}]'
	update_config '.process.args = ["stat", "-c", "%a", "/tmpfs"]'

	# Explicitly setting mode= overrides whatever mode we would've inherited.
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == "1500" ]]

	update_config '.process.args = ["cat", "/proc/self/mounts"]'
	runc run test_busybox
	[ "$status" -eq 0 ]
	grep -Ex "tmpfs /tmpfs tmpfs [^ ]*\bmode=1500\b[^ ]* .*" <<<"$output"

	# Verify that the actual directory was not chmod-ed.
	[[ "$(stat -c %a rootfs/tmpfs)" == 710 ]]
}

# https://github.com/opencontainers/runc/issues/4971
@test "runc run [tmpfs mount mode=1777 default]" {
	update_config '.mounts += [{
		type: "tmpfs",
		source: "tmpfs",
		destination: "/non-existent/foo/bar/baz",
		options: ["rw", "nodev", "nosuid"]
	}]'
	update_config '.process.args = ["stat", "-c", "%a", "/non-existent/foo/bar/baz"]'

	rm -rf rootfs/non-existent
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == "1777" ]]

	update_config '.process.args = ["cat", "/proc/self/mounts"]'

	rm -rf rootfs/non-existent
	runc run test_busybox
	[ "$status" -eq 0 ]
	# We don't explicitly set a mode= in this case, it is just the tmpfs default.
	grep -Ex "tmpfs /non-existent/foo/bar/baz tmpfs .*" <<<"$output"
	run ! grep -Ex "tmpfs /non-existent/foo/bar/baz tmpfs [^ ]*\bmode=[0-7]+\b[^ ]* .*" <<<"$output"

	# Verify that the actual modes are *not* 1777.
	[[ "$(stat -c %a rootfs/non-existent)" == 755 ]]
	[[ "$(stat -c %a rootfs/non-existent/foo)" == 755 ]]
	[[ "$(stat -c %a rootfs/non-existent/foo/bar)" == 755 ]]
	[[ "$(stat -c %a rootfs/non-existent/foo/bar/baz)" == 755 ]]
}

@test "runc run [ro /sys/fs/cgroup mounts]" {
	# Without cgroup namespace.
	update_config '.linux.namespaces -= [{"type": "cgroup"}]'
	test_ro_cgroup_mount
}

@test "runc run [ro /sys/fs/cgroup mounts + cgroupns]" {
	requires cgroupns
	# With cgroup namespace.
	update_config '.linux.namespaces |= if index({"type": "cgroup"}) then . else . + [{"type": "cgroup"}] end'
	test_ro_cgroup_mount
}

@test "runc run [mount order, container bind-mount source]" {
	test_mount_order
}

@test "runc run [mount order, container bind-mount source] (userns)" {
	test_mount_order userns
}

@test "runc run [mount order, container idmap source]" {
	test_mount_order idmap
}

@test "runc run [mount order, container idmap source] (userns)" {
	test_mount_order userns,idmap
}

@test "runc run [bind mount through a dangling symlink component]" {
	rm -rf rootfs/etc/hosts
	ln -s /tmp/foo/bar rootfs/jump
	ln -s /jump/baz/hosts rootfs/etc/hosts

	rm -rf rootfs/tmp/foo
	test_mount_target ./config.json /etc/hosts /tmp/foo/bar/baz/hosts
}

@test "runc run [bind mount through a trailing dangling symlink]" {
	rm -rf rootfs/etc/hosts
	ln -s /tmp/hosts rootfs/etc/hosts

	# File.
	rm -rf rootfs/tmp/hosts
	test_mount_target ./config.json /etc/hosts /tmp/hosts

	# Directory.
	rm -rf rootfs/tmp/hosts
	test_mount_target . /etc/hosts /tmp/hosts
}
