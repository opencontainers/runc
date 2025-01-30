#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_debian
}

function teardown() {
	teardown_bundle
}

@test "scheduler is applied" {
	update_config ' .process.scheduler = {
		"policy": "SCHED_BATCH",
		"priority": 0,
		"nice": 19
	}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_scheduler
	[ "$status" -eq 0 ]

	# Check init settings.
	runc exec test_scheduler chrt -p 1
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *"scheduling policy: SCHED_BATCH" ]]
	[[ "${lines[1]}" == *"priority: 0" ]]

	# Check exec settings derived from config.json.
	runc exec test_scheduler sh -c 'chrt -p $$'
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *"scheduling policy: SCHED_BATCH" ]]
	[[ "${lines[1]}" == *"priority: 0" ]]

	# Another exec, with different scheduler settings.
	proc='
{
	"terminal": false,
	"args": [ "/bin/sleep", "600" ],
	"cwd": "/",
	"scheduler": {
		"policy": "SCHED_DEADLINE",
		"flags": [ "SCHED_FLAG_RESET_ON_FORK" ],
		"nice": 19,
		"priority": 0,
		"runtime": 42000,
		"deadline": 100000,
		"period": 1000000
	}
}'
	__runc exec -d --pid-file pid.txt --process <(echo "$proc") test_scheduler

	run chrt -p "$(cat pid.txt)"
	[[ "${lines[0]}" == *"scheduling policy: SCHED_DEADLINE|SCHED_RESET_ON_FORK" ]]
	[[ "${lines[1]}" == *"priority: 0" ]]
	[[ "${lines[2]}" == *"runtime/deadline/period parameters: 42000/100000/1000000" ]]
}

# Checks that runc emits a specific error when scheduling policy is used
# together with specific CPUs. As documented in sched_setattr(2):
#
#   ERRORS:
#   ...
#        EPERM  The CPU affinity mask of the thread specified by pid does not
#        include all CPUs  in  the  system (see sched_setaffinity(2)).
#
@test "scheduler vs cpus" {
	requires smp

	update_config ' .linux.resources.cpu.cpus = "0"
		| .process.scheduler = {"policy": "SCHED_DEADLINE", "nice": 19, "runtime": 42000, "deadline": 1000000, "period": 1000000, }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_scheduler
	[ "$status" -eq 1 ]
	[[ "$output" == *"process scheduler can't be used together with AllowedCPUs"* ]]
}
