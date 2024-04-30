#!/usr/bin/env bats

load helpers

function setup() {
    [ $EUID -eq 0 ] && prlimit --nofile=1024:65536 -p $$
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run with RLIMIT_NOFILE(The same as system's hard value)" {
    # https://github.com/opencontainers/runc/pull/4265#discussion_r1588599809
    hard=$(ulimit -n -H)
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"hard\": ${hard}, \"soft\": ${hard}}]"
	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n"]'

	runc run test_ulimit
	[ "$status" -eq 0 ]
	[[ "${output}" == "${hard}" ]]
}

@test "runc run with RLIMIT_NOFILE(Bigger than system's hard value)" {
    # https://github.com/opencontainers/runc/pull/4265#discussion_r1588599809
    hard=$(ulimit -n -H)
    val=$(($hard+1))
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"hard\": ${val}, \"soft\": ${val}}]"
	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n"]'

	runc run test_ulimit
	[ "$status" -eq 0 ]
	[[ "${output}" == "${val}" ]]
}

@test "runc run with RLIMIT_NOFILE(Smaller than system's hard value)" {
    hard=$(ulimit -n -H)
    val=$(($hard-1))
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"hard\": ${val}, \"soft\": ${val}}]"
	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n"]'

	runc run test_ulimit
	[ "$status" -eq 0 ]
	[[ "${output}" == "${val}" ]]
}

@test "runc exec with RLIMIT_NOFILE(The same as system's hard value)" {
    hard=$(ulimit -n -H)
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"hard\": ${hard}, \"soft\": ${hard}}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox /bin/sh -c "ulimit -n"
	[ "$status" -eq 0 ]
	[[ "${output}" == "${hard}" ]]
}

@test "runc exec with RLIMIT_NOFILE(Bigger than system's hard value)" {
    hard=$(ulimit -n -H)
    val=$(($hard+1))
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"hard\": ${val}, \"soft\": ${val}}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox /bin/sh -c "ulimit -n"
	[ "$status" -eq 0 ]
	[[ "${output}" == "${val}" ]]
}

@test "runc exec with RLIMIT_NOFILE(Smaller than system's hard value)" {
    hard=$(ulimit -n -H)
    val=$(($hard-1))
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"hard\": ${val}, \"soft\": ${val}}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# issue: https://github.com/opencontainers/runc/issues/4195
	runc exec test_busybox /bin/sh -c "ulimit -n"
	[ "$status" -eq 0 ]
	[[ "${output}" == "${val}" ]]
}
