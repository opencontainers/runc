#!/usr/bin/env bats
# Exec CPU affinity tests. For more details, see:
#  - https://github.com/opencontainers/runtime-spec/pull/1253

load helpers

function setup() {
	requires smp cgroups_cpuset
	setup_busybox
}

function teardown() {
	teardown_bundle
}

function all_cpus() {
	cat /sys/devices/system/cpu/online
}

function first_cpu() {
	all_cpus | sed 's/[-,].*//g'
}

@test "runc exec [CPU affinity inherited from runc]" {
	requires root # For taskset.

	first="$(first_cpu)"

	# Container's process CPU affinity is inherited from that of runc.
	taskset -p -c "$first" $$

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

	# Check init.
	runc exec ct1 grep "Cpus_allowed_list:" /proc/1/status
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "Cpus_allowed_list:	$first" ]]

	# Check exec.
	runc exec ct1 grep "Cpus_allowed_list:" /proc/self/status
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "Cpus_allowed_list:	$first" ]]
}

@test "runc exec [CPU affinity, only initial is set]" {
	requires root # For taskset.

	first="$(first_cpu)"

	update_config ".process.execCPUAffinity.initial = \"$first\""

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

	runc exec ct1 grep "Cpus_allowed_list:" /proc/self/status
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "Cpus_allowed_list:	$first" ]]
}

@test "runc exec [CPU affinity, initial and final are set]" {
	requires root # For taskset.

	first="$(first_cpu)"
	second=$((first+1)) # Hacky; might not work in all environments.

	update_config "	  .process.execCPUAffinity.initial = \"$first\"
			| .process.execCPUAffinity.final = \"$second\""

	taskset -p -c "$first" $$
	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

	runc exec ct1 grep "Cpus_allowed_list:" /proc/self/status
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "Cpus_allowed_list:	$second" ]]
}
