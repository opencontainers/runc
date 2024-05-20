#!/usr/bin/env bats

load helpers

function setup() {
	# Do not change the Cur value to be equal to the Max value
	# Because in some environments, the soft and hard nofile limit have the same value.
	[ $EUID -eq 0 ] && prlimit --nofile=1024:65536 -p $$
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# Set and check rlimit_nofile for runc run. Arguments are:
#  $1: soft limit;
#  $2: hard limit.
function run_check_nofile() {
	soft="$1"
	hard="$2"
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"soft\": ${soft}, \"hard\": ${hard}}]"
	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n; ulimit -H -n"]'

	runc run test_rlimit
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "${soft}" ]]
	[[ "${lines[1]}" == "${hard}" ]]
}

# Set and check rlimit_nofile for runc exec. Arguments are:
#  $1: soft limit;
#  $2: hard limit.
function exec_check_nofile() {
	soft="$1"
	hard="$2"
	update_config ".process.rlimits = [{\"type\": \"RLIMIT_NOFILE\", \"soft\": ${soft}, \"hard\": ${hard}}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_rlimit
	[ "$status" -eq 0 ]

	runc exec test_rlimit /bin/sh -c "ulimit -n; ulimit -H -n"
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "${soft}" ]]
	[[ "${lines[1]}" == "${hard}" ]]
}

@test "runc run with RLIMIT_NOFILE(The same as system's hard value)" {
	hard=$(ulimit -n -H)
	soft="$hard"
	run_check_nofile "$soft" "$hard"
}

@test "runc run with RLIMIT_NOFILE(Bigger than system's hard value)" {
	requires root
	limit=$(ulimit -n -H)
	soft=$((limit + 1))
	hard=$soft
	run_check_nofile "$soft" "$hard"
}

@test "runc run with RLIMIT_NOFILE(Smaller than system's hard value)" {
	limit=$(ulimit -n -H)
	soft=$((limit - 1))
	hard=$soft
	run_check_nofile "$soft" "$hard"
}

@test "runc exec with RLIMIT_NOFILE(The same as system's hard value)" {
	hard=$(ulimit -n -H)
	soft="$hard"
	exec_check_nofile "$soft" "$hard"
}

@test "runc exec with RLIMIT_NOFILE(Bigger than system's hard value)" {
	requires root
	limit=$(ulimit -n -H)
	soft=$((limit + 1))
	hard=$soft
	exec_check_nofile "$soft" "$hard"
}

@test "runc exec with RLIMIT_NOFILE(Smaller than system's hard value)" {
	limit=$(ulimit -n -H)
	soft=$((limit - 1))
	hard=$soft
	exec_check_nofile "$soft" "$hard"
}
