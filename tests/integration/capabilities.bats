#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/cat", "/proc/self/status"]'
}

function teardown() {
	teardown_bundle
}

@test "runc run no capability" {
	runc run test_no_caps
	[ "$status" -eq 0 ]

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}

@test "runc run with unknown capability" {
	update_config '.process.capabilities.bounding = ["CAP_UNKNOWN", "UNKNOWN_CAP"]'
	runc run test_unknown_caps
	[ "$status" -eq 0 ]

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}

@test "runc run with new privileges" {
	update_config '.process.noNewPrivileges = false'
	runc run test_new_privileges
	[ "$status" -eq 0 ]

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
	[[ "${output}" == *"NoNewPrivs:	0"* ]]
}

@test "runc run with some capabilities" {
	update_config '.process.user = {"uid":0}'
	update_config '.process.capabilities.bounding = ["CAP_SYS_ADMIN"]'
	update_config '.process.capabilities.permitted = ["CAP_SYS_ADMIN", "CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"]'
	runc run test_some_caps
	[ "$status" -eq 0 ]

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapBnd:	0000000000200000"* ]]
	[[ "${output}" == *"CapEff:	0000000000200000"* ]]
	[[ "${output}" == *"CapPrm:	0000000000200000"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}
