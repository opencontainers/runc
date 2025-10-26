#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function test_events() {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	set_cgroups_path

	local status interval retry_every=1
	if [ $# -eq 2 ]; then
		interval="$1"
		retry_every="$2"
	fi

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	# Spawn two subshels:
	# 1. Event logger that sends stats events to events.log.
	(__runc events ${interval:+ --interval "$interval"} test_busybox >events.log) &
	# 2. Waits for an event that includes test_busybox then kills the
	#    test_busybox container which causes the event logger to exit.
	(
		retry 10 "$retry_every" grep -q test_busybox events.log
		__runc delete -f test_busybox
	) &
	wait # for both subshells to finish

	[ -e events.log ]

	output=$(head -1 events.log)
	[[ "$output" == [\{]"\"type\""[:]"\"stats\""[,]"\"id\""[:]"\"test_busybox\""[,]* ]]
	[[ "$output" == *"data"* ]]
}

@test "events --stats" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	init_cgroup_paths

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	# generate stats
	runc -0 events --stats test_busybox
	[[ "${lines[0]}" == [\{]"\"type\""[:]"\"stats\""[,]"\"id\""[:]"\"test_busybox\""[,]* ]]
	[[ "${lines[0]}" == *"data"* ]]
}

@test "events --stats with psi data" {
	# XXX: CPU PSI avg data only available to root.
	requires root cgroups_v2 psi
	init_cgroup_paths

	update_config '.linux.resources.cpu |= { "quota": 1000 }'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	# Stress the CPU a bit. Need something that runs for more than 10s.
	runc -0 exec test_busybox dd if=/dev/zero bs=1 count=128K of=/dev/null

	runc exec test_busybox sh -c 'tail /sys/fs/cgroup/*.pressure'

	runc -0 events --stats test_busybox

	# Check PSI metrics.
	jq '.data.cpu.psi' <<<"${lines[0]}"
	for psi_type in some full; do
		for psi_metric in avg10 avg60 avg300 total; do
			echo -n "checking .data.cpu.psi.$psi_type.$psi_metric != 0: "
			jq -e '.data.cpu.psi.'$psi_type.$psi_metric' != 0' <<<"${lines[0]}"
		done
	done
}

# See https://github.com/opencontainers/cgroups/pull/24
@test "events --stats with hugetlb" {
	requires cgroups_v2 cgroups_hugetlb
	init_cgroup_paths

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	runc -0 events --stats test_busybox
	# Ensure hugetlb node is present.
	jq -e '.data.hugetlb // empty' <<<"${lines[0]}"
}

@test "events --interval default" {
	test_events
}

@test "events --interval 1s" {
	test_events 1s 1
}

@test "events --interval 100ms" {
	test_events 100ms 0.1
}

@test "events oom" {
	# XXX: oom is not triggered for rootless containers.
	requires root cgroups_swap
	init_cgroup_paths

	# we need the container to hit OOM, so disable swap
	update_config '(.. | select(.resources? != null)) .resources.memory |= {"limit": 33554432, "swap": 33554432}'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	# spawn two sub processes (shells)
	# the first sub process is an event logger that sends stats events to events.log
	# the second sub process exec a memory hog process to cause a oom condition
	# and waits for an oom event
	(__runc events test_busybox >events.log) &
	(
		retry 10 1 grep -q test_busybox events.log
		# shellcheck disable=SC2016
		__runc exec -d test_busybox sh -c 'test=$(dd if=/dev/urandom ibs=5120k)'
		retry 30 1 grep -q oom events.log
		__runc delete -f test_busybox
	) &
	wait # wait for the above sub shells to finish

	grep -q '{"type":"oom","id":"test_busybox"}' events.log
}
