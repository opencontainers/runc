#!/usr/bin/env bats

load helpers

function setup() {
	requires root

	setup_debian

	mkdir -p rootfs/{proc,sys,tmp}
	mkdir -p rootfs/tmp/mount1

	update_config ' .mounts += [
					{
						"source": "tmpfs",
						"destination": "/tmp/mount1",
						"type": "tmpfs"
					}
				] '
}

function teardown() {
	teardown_bundle
}

# https://github.com/opencontainers/runc/issues/1755
@test "runc run [tmpfs mount propagation]" {
	update_config ' .process.args = ["sh", "-c", "findmnt --noheadings -o PROPAGATION /tmp/mount1"]'
	# Add the shared option to the mount
	update_config ' .mounts |= map((select(.destination == "/tmp/mount1") | .options += ["shared"]) // .)'

	runc run test_debian
	[ "$status" -eq 0 ]
	[[ "$output" == *"shared"* ]]
}
