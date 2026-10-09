#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_debian
}

function teardown() {
	teardown_bundle
}

@test "runc run [rootfsPropagation shared]" {
	update_config ' .linux.rootfsPropagation = "shared" '

	update_config ' .process.args = ["findmnt", "--noheadings", "-o", "PROPAGATION", "/"] '

	run -0 runc run test_shared_rootfs
	assert_output "shared"
}

@test "runc run [rootfsPropagation, host mount ns] must fail" {
	update_config '	  .linux.rootfsPropagation = "rslave"
			| .linux.namespaces -= [{"type": "mount"}]
			| .linux.maskedPaths = []
			| .linux.readonlyPaths = []
			| .root.readonly = false'

	run -1 runc run test_host_mntns
	assert_output --partial "unable to set rootfs propagation without a private MNT namespace"
}
