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
	update_config ' .process.args |= ["chrt", "-p", "1"]
            | .process.scheduler = {"policy": "SCHED_DEADLINE", "runtime": 42000, "deadline": 1000000, "period": 1000000, }'

	runc run test_scheduler
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *"scheduling policy: SCHED_DEADLINE"* ]]
	[[ "${lines[1]}" == *"priority: 0"* ]]
	[[ "${lines[2]}" == *"runtime/deadline/period parameters: 42000/1000000/1000000"* ]]
}
