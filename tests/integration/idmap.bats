#!/usr/bin/env bats

load helpers

function setup() {
	OVERFLOW_UID="$(cat /proc/sys/kernel/overflowuid)"
	OVERFLOW_GID="$(cat /proc/sys/kernel/overflowgid)"
	requires root
	requires_kernel 5.12

	setup_debian
	requires_idmap_fs .

	# Prepare source folders for mounts.
	mkdir -p source-{1,2,multi{1,2,3}}/
	touch source-{1,2,multi{1,2,3}}/foo.txt
	touch source-multi{1,2,3}/{bar,baz}.txt

	# Change the owners for everything other than source-1.
	chown 1:1 source-2/foo.txt

	# A source with multiple users owning files.
	chown 100:211 source-multi1/foo.txt
	chown 101:222 source-multi1/bar.txt
	chown 102:233 source-multi1/baz.txt

	# Same gids as multi1, different uids.
	chown 200:211 source-multi2/foo.txt
	chown 201:222 source-multi2/bar.txt
	chown 202:233 source-multi2/baz.txt

	# Even more users -- 1000 uids, 500 gids.
	chown 5000528:6000491 source-multi3/foo.txt
	chown 5000133:6000337 source-multi3/bar.txt
	chown 5000999:6000444 source-multi3/baz.txt

	# Add a symlink-containing source.
	ln -s source-multi1 source-multi1-symlink

	# Add some top-level files in the mount tree.
	mkdir -p mnt-subtree/multi{1,2}
	touch mnt-subtree/{foo,bar,baz}.txt
	chown 100:211 mnt-subtree/foo.txt
	chown 200:222 mnt-subtree/bar.txt
	chown 300:233 mnt-subtree/baz.txt

	mounts_file="$PWD/.all-mounts"
	echo -n >"$mounts_file"
}

function teardown() {
	if [ -v mounts_file ]; then
		xargs -n 1 -a "$mounts_file" -- umount -l
		rm -f "$mounts_file"
	fi
	teardown_bundle
}

function setup_host_bind_mount() {
	src="$1"
	dst="$2"

	mount --bind "$src" "$dst"
	echo "$dst" >>"$mounts_file"
}

function setup_idmap_userns() {
	update_config '.linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
		| .linux.gidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]'
	remap_rootfs
}

function setup_bind_mount() {
	mountname="${1:-1}"
	update_config '.mounts += [
			{
				"source": "source-'"$mountname"'/",
				"destination": "/tmp/bind-mount-'"$mountname"'",
				"options": ["bind"]
			}
		]'
}

function setup_idmap_single_mount() {
	uidmap="$1" # ctr:host:size
	gidmap="$2" # ctr:host:size
	mountname="$3"
	destname="${4:-$mountname}"

	read -r uid_containerID uid_hostID uid_size <<<"$(tr : ' ' <<<"$uidmap")"
	read -r gid_containerID gid_hostID gid_size <<<"$(tr : ' ' <<<"$gidmap")"

	update_config '.mounts += [
			{
				"source": "source-'"$mountname"'/",
				"destination": "/tmp/mount-'"$destname"'",
				"options": ["bind"],
				"uidMappings": [{"containerID": '"$uid_containerID"', "hostID": '"$uid_hostID"', "size": '"$uid_size"'}],
				"gidMappings": [{"containerID": '"$gid_containerID"', "hostID": '"$gid_hostID"', "size": '"$gid_size"'}]
			}
		]'
}

function setup_idmap_basic_mount() {
	mountname="${1:-1}"
	setup_idmap_single_mount 0:100000:65536 0:100000:65536 "$mountname"
}

@test "simple idmap mount [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=0=0="* ]]
}

@test "simple idmap mount [no userns]" {
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=100000=100000="* ]]
}

@test "write to an idmap mount [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "touch /tmp/mount-1/bar && stat -c =%u=%g= /tmp/mount-1/bar"]'

	runc -0 run test_debian
	[[ "$output" == *"=0=0="* ]]
}

@test "write to an idmap mount [no userns]" {
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "touch /tmp/mount-1/bar && stat -c =%u=%g= /tmp/mount-1/bar"]'

	runc ! run test_debian
	# The write must fail because the user is unmapped.
	[[ "$output" == *"Value too large for defined data type"* ]] # ERANGE
}

@test "idmap mount with propagation flag [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "findmnt -o PROPAGATION /tmp/mount-1"]'
	# Add the shared option to the idmap mount.
	update_config '.mounts |= map((select(.source == "source-1/") | .options += ["shared"]) // .)'

	runc -0 run test_debian
	[[ "$output" == *"shared"* ]]
}

@test "idmap mount with relative path [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount

	update_config '.process.args = ["sh", "-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt"]'
	# Switch the mount to have a relative mount destination.
	update_config '.mounts |= map((select(.source == "source-1/") | .destination = "tmp/mount-1") // .)'

	runc -0 run test_debian
	[[ "$output" == *"=0=0="* ]]
}

@test "idmap mount with bind mount [userns]" {
	setup_idmap_userns
	setup_idmap_basic_mount
	setup_bind_mount

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/{,bind-}mount-1/foo.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-1/foo.txt:0=0="* ]]
	[[ "$output" == *"=/tmp/bind-mount-1/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
}

@test "idmap mount with bind mount [no userns]" {
	setup_idmap_basic_mount
	setup_bind_mount

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/{,bind-}mount-1/foo.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-1/foo.txt:100000=100000="* ]]
	[[ "$output" == *"=/tmp/bind-mount-1/foo.txt:0=0="* ]]
}

@test "two idmap mounts (same mapping) with two bind mounts [userns]" {
	setup_idmap_userns

	setup_idmap_basic_mount 1
	setup_bind_mount 1
	setup_bind_mount 2
	setup_idmap_basic_mount 2

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-[12]/foo.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-1/foo.txt:0=0="* ]]
	[[ "$output" == *"=/tmp/mount-2/foo.txt:1=1="* ]]
}

@test "same idmap mount (different mappings) [userns]" {
	setup_idmap_userns

	# Mount the same directory with different mappings. Make sure we also use
	# different mappings for uids and gids.
	setup_idmap_single_mount 100:100000:100 200:100000:100 multi1
	setup_idmap_single_mount 100:101000:100 200:102000:100 multi1 multi1-alt
	setup_idmap_single_mount 100:102000:100 200:103000:100 multi1-symlink multi1-alt-sym

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1{,-alt{,-sym}}/{foo,bar,baz}.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:0=11="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:1=22="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:2=33="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/foo.txt:1000=2011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/bar.txt:1001=2022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/baz.txt:1002=2033="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt-sym/foo.txt:2000=3011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt-sym/bar.txt:2001=3022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt-sym/baz.txt:2002=3033="* ]]
}

@test "same idmap mount (different mappings) [no userns]" {
	# Mount the same directory with different mappings. Make sure we also use
	# different mappings for uids and gids.
	setup_idmap_single_mount 100:100000:100 200:100000:100 multi1
	setup_idmap_single_mount 100:101000:100 200:102000:100 multi1 multi1-alt
	setup_idmap_single_mount 100:102000:100 200:103000:100 multi1-symlink multi1-alt-sym

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1{,-alt{,-sym}}/{foo,bar,baz}.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:100000=100011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:100001=100022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:100002=100033="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/foo.txt:101000=102011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/bar.txt:101001=102022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt/baz.txt:101002=102033="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt-sym/foo.txt:102000=103011="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt-sym/bar.txt:102001=103022="* ]]
	[[ "$output" == *"=/tmp/mount-multi1-alt-sym/baz.txt:102002=103033="* ]]
}

@test "multiple idmap mounts (different mappings) [userns]" {
	setup_idmap_userns

	# Make sure we use different mappings for uids and gids.
	setup_idmap_single_mount 100:101100:3 200:101900:50 multi1
	setup_idmap_single_mount 200:102200:3 200:102900:100 multi2
	setup_idmap_single_mount 5000000:103000:1000 6000000:103000:500 multi3

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi[123]/{foo,bar,baz}.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1100=1911="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:1101=1922="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:1102=1933="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/foo.txt:2200=2911="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/bar.txt:2201=2922="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/baz.txt:2202=2933="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/foo.txt:3528=3491="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/bar.txt:3133=3337="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/baz.txt:3999=3444="* ]]
}

@test "multiple idmap mounts (different mappings) [no userns]" {
	# Make sure we use different mappings for uids and gids.
	setup_idmap_single_mount 100:1100:3 200:1900:50 multi1
	setup_idmap_single_mount 200:2200:3 200:2900:100 multi2
	setup_idmap_single_mount 5000000:3000:1000 6000000:3000:500 multi3

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi[123]/{foo,bar,baz}.txt"]'

	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1100=1911="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:1101=1922="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:1102=1933="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/foo.txt:2200=2911="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/bar.txt:2201=2922="* ]]
	[[ "$output" == *"=/tmp/mount-multi2/baz.txt:2202=2933="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/foo.txt:3528=3491="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/bar.txt:3133=3337="* ]]
	[[ "$output" == *"=/tmp/mount-multi3/baz.txt:3999=3444="* ]]
}

@test "idmap mount (complicated mapping) [userns]" {
	setup_idmap_userns

	update_config '.mounts += [
			{
				"source": "source-multi1/",
				"destination": "/tmp/mount-multi1",
				"options": ["bind"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 1},
					{"containerID": 101, "hostID": 102000, "size": 1},
					{"containerID": 102, "hostID": 103000, "size": 1}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:3000=3303="* ]]
}

@test "idmap mount (complicated mapping) [no userns]" {
	update_config '.mounts += [
			{
				"source": "source-multi1/",
				"destination": "/tmp/mount-multi1",
				"options": ["bind"],
				"uidMappings": [
					{"containerID": 100, "hostID": 1000, "size": 1},
					{"containerID": 101, "hostID": 2000, "size": 1},
					{"containerID": 102, "hostID": 3000, "size": 1}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 1100, "size": 10},
					{"containerID": 220, "hostID": 2200, "size": 10},
					{"containerID": 230, "hostID": 3300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-multi1/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-multi1/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-multi1/baz.txt:3000=3303="* ]]
}

@test "idmap mount (non-recursive idmap) [userns]" {
	setup_idmap_userns

	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 3},
					{"containerID": 200, "hostID": 102000, "size": 3},
					{"containerID": 300, "hostID": 103000, "size": 3}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:3000=3303="* ]]
	# Because we used "idmap", the child mounts were not remapped recursively.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
}

@test "idmap mount (non-recursive idmap) [no userns]" {
	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 3},
					{"containerID": 200, "hostID": 102000, "size": 3},
					{"containerID": 300, "hostID": 103000, "size": 3}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:101000=101101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:102000=102202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:103000=103303="* ]]
	# Because we used "idmap", the child mounts were not remapped recursively.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:100=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:101=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:102=233="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:200=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:201=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:202=233="* ]]
}

@test "idmap mount (idmap flag) [userns]" {
	setup_idmap_userns

	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "idmap"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 3},
					{"containerID": 200, "hostID": 102000, "size": 3},
					{"containerID": 300, "hostID": 103000, "size": 3}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:3000=3303="* ]]
	# Because we used "idmap", the child mounts were not remapped recursively.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
}

@test "idmap mount (idmap flag) [no userns]" {
	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "idmap"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 3},
					{"containerID": 200, "hostID": 102000, "size": 3},
					{"containerID": 300, "hostID": 103000, "size": 3}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:101000=101101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:102000=102202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:103000=103303="* ]]
	# Because we used "idmap", the child mounts were not remapped recursively.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:100=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:101=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:102=233="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:200=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:201=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:202=233="* ]]
}

@test "idmap mount (ridmap flag) [userns]" {
	setup_idmap_userns

	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "ridmap"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 3},
					{"containerID": 200, "hostID": 102000, "size": 3},
					{"containerID": 300, "hostID": 103000, "size": 3}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:2000=2202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:3000=3303="* ]]
	# The child mounts have the same mapping applied.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:1000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:1001=2202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:1002=3303="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:2000=1101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:2001=2202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:2002=3303="* ]]
}

@test "idmap mount (ridmap flag) [no userns]" {
	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "ridmap"],
				"uidMappings": [
					{"containerID": 100, "hostID": 101000, "size": 3},
					{"containerID": 200, "hostID": 102000, "size": 3},
					{"containerID": 300, "hostID": 103000, "size": 3}
				],
				"gidMappings": [
					{"containerID": 210, "hostID": 101100, "size": 10},
					{"containerID": 220, "hostID": 102200, "size": 10},
					{"containerID": 230, "hostID": 103300, "size": 10}
				]
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:101000=101101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:102000=102202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:103000=103303="* ]]
	# The child mounts have the same mapping applied.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:101000=101101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:101001=102202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:101002=103303="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:102000=101101="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:102001=102202="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:102002=103303="* ]]
}

@test "idmap mount (idmap flag, implied mapping) [userns]" {
	setup_idmap_userns

	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "idmap"],
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:100=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:200=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:300=233="* ]]
	# Because we used "idmap", the child mounts were not remapped recursively.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
}

@test "idmap mount (ridmap flag, implied mapping) [userns]" {
	setup_idmap_userns

	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "ridmap"],
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:100=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:200=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:300=233="* ]]
	# The child mounts have the same mapping applied.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:100=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:101=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:102=233="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:200=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:201=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:202=233="* ]]
}

@test "idmap mount (idmap flag, implied mapping, userns join) [userns]" {
	# Create a detached container with the id-mapping we want.
	cp config.json config.json.bak
	update_config '.linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
		| .linux.gidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]'
	update_config '.process.args = ["sleep", "infinity"]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" target_userns

	# Configure our container to attach to the first container's userns.
	target_pid="$(__runc state target_userns | jq .pid)"
	update_config '.linux.namespaces |= map(if .type == "user" then (.path = "/proc/'"$target_pid"'/ns/" + .type) else . end)'
	update_config 'del(.linux.uidMappings) | del(.linux.gidMappings)'

	setup_host_bind_mount "source-multi1/" "mnt-subtree/multi1"
	setup_host_bind_mount "source-multi2/" "mnt-subtree/multi2"

	update_config '.mounts += [
			{
				"source": "mnt-subtree/",
				"destination": "/tmp/mount-tree",
				"options": ["rbind", "idmap"],
			}
		]'

	update_config '.process.args = ["bash", "-c", "stat -c =%n:%u=%g= /tmp/mount-tree{,/multi1,/multi2}/{foo,bar,baz}.txt"]'
	runc -0 run test_debian
	[[ "$output" == *"=/tmp/mount-tree/foo.txt:100=211="* ]]
	[[ "$output" == *"=/tmp/mount-tree/bar.txt:200=222="* ]]
	[[ "$output" == *"=/tmp/mount-tree/baz.txt:300=233="* ]]
	# Because we used "idmap", the child mounts were not remapped recursively.
	[[ "$output" == *"=/tmp/mount-tree/multi1/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi1/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/foo.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/bar.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
	[[ "$output" == *"=/tmp/mount-tree/multi2/baz.txt:$OVERFLOW_UID=$OVERFLOW_GID="* ]]
}
