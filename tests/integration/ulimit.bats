#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run with RLIMIT_NOFILE" {
	update_config '.process.rlimits = [{"type": "RLIMIT_NOFILE", "hard": 65536, "soft": 65536}]'

	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n"]'
	runc run test_ulimit
	[ "$status" -eq 0 ]
	[[ "${output}" == "65536" ]]
}

@test "runc exec with RLIMIT_NOFILE" {
	update_config '	 .process.rlimits = [{"type": "RLIMIT_NOFILE", "hard": 2000, "soft": 1000}]'

	runc run -d --console-socket "$CONSOLE_SOCKET" nofile
	[ "$status" -eq 0 ]

	for ((i = 0; i < 100; i++)); do
		runc exec nofile sh -c 'ulimit -n'
		echo "[$i] $output"
		[[ "${output}" == "1000" ]]
	done
}

@test "runc run+exec two containers with RLIMIT_NOFILE" {
	update_config '.process.capabilities.bounding = ["CAP_SYS_RESOURCE"]'
	update_config '.process.rlimits = [{"type": "RLIMIT_NOFILE", "hard": 65536, "soft": 65536}]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n"]'
	runc run test_ulimit
	[ "$status" -eq 0 ]
	[[ "${output}" == "65536" ]]

	# issue: https://github.com/opencontainers/runc/issues/4195
	runc exec test_busybox /bin/sh -c "ulimit -n"
	[ "$status" -eq 0 ]
	[[ "${output}" == "65536" ]]
}
