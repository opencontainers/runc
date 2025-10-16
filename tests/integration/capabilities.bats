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
	runc -0 run test_no_caps

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}

@test "runc run with unknown capability" {
	update_config '.process.capabilities.bounding = ["CAP_UNKNOWN", "UNKNOWN_CAP"]'
	runc -0 run test_unknown_caps

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}

@test "runc run with new privileges" {
	update_config '.process.noNewPrivileges = false'
	runc -0 run test_new_privileges

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
	[[ "${output}" == *"NoNewPrivs:	0"* ]]
}

@test "runc run with some capabilities" {
	update_config '.process.user = {"uid":0}'
	update_config '.process.capabilities.bounding = ["CAP_SYS_ADMIN"]'
	update_config '.process.capabilities.permitted = ["CAP_SYS_ADMIN", "CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"]'
	runc -0 run test_some_caps

	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapBnd:	0000000000200000"* ]]
	[[ "${output}" == *"CapEff:	0000000000200000"* ]]
	[[ "${output}" == *"CapPrm:	0000000000200000"* ]]
	[[ "${output}" == *"NoNewPrivs:	1"* ]]
}

@test "runc exec --cap" {
	update_config '	  .process.args = ["/bin/sh"]
			| .process.capabilities = {}'
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_exec_cap

	runc -0 exec test_exec_cap cat /proc/self/status
	# Check no capabilities are set.
	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapPrm:	0000000000000000"* ]]
	[[ "${output}" == *"CapEff:	0000000000000000"* ]]
	[[ "${output}" == *"CapBnd:	0000000000000000"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]

	runc -0 exec --cap CAP_KILL --cap CAP_AUDIT_WRITE test_exec_cap cat /proc/self/status
	# Check capabilities are added into bounding/effective/permitted only,
	# but not to inheritable or ambient.
	#
	# CAP_KILL is 5, the bit mask is 0x20 (1 << 5).
	# CAP_AUDIT_WRITE is 26, the bit mask is 0x20000000 (1 << 26).
	[[ "${output}" == *"CapInh:	0000000000000000"* ]]
	[[ "${output}" == *"CapPrm:	0000000020000020"* ]]
	[[ "${output}" == *"CapEff:	0000000020000020"* ]]
	[[ "${output}" == *"CapBnd:	0000000020000020"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000000"* ]]
}

@test "runc exec --cap [ambient is set from spec]" {
	update_config '	  .process.args = ["/bin/sh"]
			| .process.capabilities.inheritable = ["CAP_CHOWN", "CAP_SYSLOG"]
			| .process.capabilities.permitted = ["CAP_KILL", "CAP_CHOWN"]
			| .process.capabilities.effective = ["CAP_KILL"]
			| .process.capabilities.bounding = ["CAP_KILL", "CAP_CHOWN", "CAP_SYSLOG"]
			| .process.capabilities.ambient = ["CAP_CHOWN"]'
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_some_caps

	runc -0 exec test_some_caps cat /proc/self/status
	# Check that capabilities are as set in spec.
	#
	# CAP_CHOWN is 0, the bit mask is 0x1 (1 << 0)
	# CAP_KILL is 5, the bit mask is 0x20 (1 << 5).
	# CAP_SYSLOG is 34, the bit mask is 0x400000000 (1 << 34).
	[[ "${output}" == *"CapInh:	0000000400000001"* ]]
	[[ "${output}" == *"CapPrm:	0000000000000021"* ]]
	[[ "${output}" == *"CapEff:	0000000000000021"* ]]
	[[ "${output}" == *"CapBnd:	0000000400000021"* ]]
	[[ "${output}" == *"CapAmb:	0000000000000001"* ]]

	# Check that if config.json has an inheritable capability set,
	# runc exec --cap adds ambient capabilities.
	runc -0 exec --cap CAP_SYSLOG test_some_caps cat /proc/self/status
	[[ "${output}" == *"CapInh:	0000000400000001"* ]]
	[[ "${output}" == *"CapPrm:	0000000400000021"* ]]
	[[ "${output}" == *"CapEff:	0000000400000021"* ]]
	[[ "${output}" == *"CapBnd:	0000000400000021"* ]]
	[[ "${output}" == *"CapAmb:	0000000400000001"* ]]
}

@test "runc run [ambient caps not set in inheritable result in a warning]" {
	update_config '   .process.capabilities.inheritable = ["CAP_KILL"]
                       | .process.capabilities.ambient = ["CAP_KILL", "CAP_CHOWN"]'
	runc -0 run test_amb
	# This should result in CAP_KILL set in ambient,
	# and a warning about inability to set CAP_CHOWN.
	#
	# CAP_CHOWN is 0, the bit mask is 0x1 (1 << 0)
	# CAP_KILL is 5, the bit mask is 0x20 (1 << 5).
	[[ "$output" == *"can't raise ambient capability CAP_CHOWN: "* ]]
	[[ "${output}" == *"CapAmb:	0000000000000020"* ]]
}
