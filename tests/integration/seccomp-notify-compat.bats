#!/usr/bin/env bats

load helpers

function setup() {
	if is_kernel_gte 5.6; then
		skip "requires kernel < 5.6"
	fi

	requires arch_x86_64

	setup_seccompagent
	setup_busybox
}

function teardown() {
	teardown_seccompagent
	teardown_bundle
}

# Support for seccomp notify requires Linux > 5.6, check that on older kernels
# return an error.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY old kernel)" {
	# Use just any seccomp profile with a notify action.
	update_config ' .linux.seccomp = {
				"defaultAction": "SCMP_ACT_ALLOW",
				"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
				"architectures": [ "SCMP_ARCH_X86","SCMP_ARCH_X32", "SCMP_ARCH_X86_64" ],
				"syscalls": [{ "names": [ "mkdir" ], "action": "SCMP_ACT_NOTIFY" }]
			}'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"seccomp notify unsupported:"* ]]
}
