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

@test "runc exec --cap" {
	update_config '.process.terminal = false'
	update_config '.process.args = ["sleep", "infinity"]'
	update_config '.process.user = {"uid":0}'
	update_config '.process.capabilities.bounding = ["CAP_KILL", "CAP_CHOWN", "CAP_SYSLOG"]'
	update_config '.process.capabilities.effective = ["CAP_KILL"]'
	update_config '.process.capabilities.permitted = ["CAP_KILL", "CAP_CHOWN"]'
	update_config '.process.capabilities.inheritable = ["CAP_CHOWN", "CAP_SYSLOG"]'
	update_config '.process.capabilities.ambient = ["CAP_CHOWN"]'
	__runc run -d test_some_caps
	[ "$status" -eq 0 ]

	runc exec test_some_caps /bin/cat /proc/self/status
	[[ "${output}" == *"CapInh:	0000000400000001"* ]]
	[[ "${output}" == *"CapBnd:	0000000400000021"* ]]
	[[ "${output}" == *"CapEff:	0000000000000021"* ]]
	[[ "${output}" == *"CapPrm:	0000000000000021"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000001"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]

	runc exec --cap CAP_SYSLOG test_some_caps /bin/cat /proc/self/status
	[[ "${output}" == *"CapInh:	0000000400000001"* ]]
	[[ "${output}" == *"CapBnd:	0000000400000021"* ]]
	[[ "${output}" == *"CapEff:	0000000400000021"* ]]
	[[ "${output}" == *"CapPrm:	0000000400000021"* ]]
	[[ "${output}" == *"CapAmb:	0000000400000001"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}
