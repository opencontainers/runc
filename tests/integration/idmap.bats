#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	requires_kernel 5.12
	requires_idmap_fs /tmp

	setup_debian

	# Prepare source folders for bind mount
	mkdir -p source-{1,2}/
	touch source-{1,2}/foo.txt

	# Use other owner for source-2
	chown 1:1 source-2/foo.txt

	mkdir -p rootfs/{proc,sys,tmp}
	mkdir -p rootfs/tmp/mount-{1,2}
	mkdir -p rootfs/mnt/bind-mount-{1,2}

	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65536}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65536}]
		| .process.args += ["-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt"]
		| .mounts += [
					{
						"source": "source-1/",
						"destination": "/tmp/mount-1",
						"options": ["bind"],
						"uidMappings": [ {
						                  "containerID": 0,
						                  "hostID": 100000,
						                  "size": 65536
						                }
						],
						"gidMappings": [        {
						                  "containerID": 0,
						                  "hostID": 100000,
						                  "size": 65536
						                }
						]
					}
				] '
}

function teardown() {
	teardown_bundle
}

@test "simple idmap mount" {
	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
}

@test "write to an idmap mount" {
	update_config ' .process.args = ["sh", "-c", "touch /tmp/mount-1/bar && stat -c =%u=%g= /tmp/mount-1/bar"]'
	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
}

@test "idmap mount with propagation flag" {
	update_config ' .process.args = ["sh", "-c", "findmnt -o PROPAGATION /tmp/mount-1"]'
	# Add the shared option to the idmap mount
	update_config ' .mounts |= map((select(.source == "source-1/") | .options += ["shared"]) // .)'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"shared"* ]]
}

@test "idmap mount with relative path" {
	update_config ' .mounts |= map((select(.source == "source-1/") | .destination = "tmp/mount-1") // .)'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
}

@test "idmap mount with bind mount" {
	update_config '   .mounts += [
					{
						"source": "source-2/",
						"destination": "/tmp/mount-2",
						"options": ["bind"]
					}
				] '

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
}

@test "two idmap mounts with two bind mounts" {
	update_config '   .process.args = ["sh", "-c", "stat -c =%u=%g= /tmp/mount-1/foo.txt /tmp/mount-2/foo.txt"]
			| .mounts += [
					{
						"source": "source-1/",
						"destination": "/mnt/bind-mount-1",
						"options": ["bind"]
					},
					{
						"source": "source-2/",
						"destination": "/mnt/bind-mount-2",
						"options": ["bind"]
					},
					{
						"source": "source-2/",
						"destination": "/tmp/mount-2",
						"options": ["bind"],
						"uidMappings": [ {
						                  "containerID": 0,
						                  "hostID": 100000,
						                  "size": 65536
						                }
						],
						"gidMappings": [        {
						                  "containerID": 0,
						                  "hostID": 100000,
						                  "size": 65536
						                }
						]
					}
				] '

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"=0=0="* ]]
	# source-2/ is owned by 1:1, so we expect this with the idmap mount too.
	[[ "$output" == *"=1=1="* ]]
}

@test "idmap mount without gidMappings fails" {
	update_config ' .mounts |= map((select(.source == "source-1/") | del(.gidMappings) ) // .)'

	runc run test_debian
	[ "$status" -eq 1 ]
	[[ "${output}" == *"invalid mount"* ]]
}

@test "idmap mount without uidMappings fails" {
	update_config ' .mounts |= map((select(.source == "source-1/") | del(.uidMappings) ) // .)'

	runc run test_debian
	[ "$status" -eq 1 ]
	[[ "${output}" == *"invalid mount"* ]]
}

@test "idmap mount without bind fails" {
	update_config ' .mounts |= map((select(.source == "source-1/") | .options = [""]) // .)'

	runc run test_debian
	[ "$status" -eq 1 ]
	[[ "${output}" == *"invalid mount"* ]]
}

@test "idmap mount with different mapping than userns fails" {
	# Let's modify the containerID to be 1, instead of 0 as it is in the
	# userns config.
	update_config ' .mounts |= map((select(.source == "source-1/") | .uidMappings[0]["containerID"] = 1 ) // .)'

	runc run test_debian
	[ "$status" -eq 1 ]
	[[ "${output}" == *"invalid mount"* ]]
}
