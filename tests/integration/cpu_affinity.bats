#!/usr/bin/env bats
# CPU affinity tests. For more details, see:
#  - https://github.com/opencontainers/runtime-spec/pull/1253
#  - https://github.com/opencontainers/runtime-spec/pull/1296

load helpers

INITIAL_CPU_MASK="$(grep -F Cpus_allowed_list: /proc/self/status | awk '{ print $2 }')"

function setup() {
	requires smp cgroups_cpuset
	setup_busybox

	echo "Initial CPU mask: $INITIAL_CPU_MASK" >&2
	echo "---" >&2
}

function teardown() {
	teardown_bundle
}

function first_cpu() {
	sed 's/[-,].*//g' </sys/devices/system/cpu/online
}

# Convert list of cpus ("0,1" or "0-1") to mask as printed by nsexec.
# NOTE the range conversion is not proper, merely sufficient for tests here.
function cpus_to_mask() {
	local cpus=$* mask=0

	cpus=${cpus//,/-} # 1. "," --> "-".
	cpus=${cpus//-/ } # 2. "-" --> " ".

	for c in $cpus; do
		mask=$((mask | 1 << c))
	done

	printf "0x%x" $mask
}

# Checks runc specific debug messages.
# bats test_tags=runc-specific
@test "runc exec [CPU affinity, only initial set from process.json]" {
	first="$(first_cpu)"
	second=$((first + 1)) # Hacky; might not work in all environments.

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" ct1

	for cpus in "$second" "$first-$second" "$first,$second" "$first"; do
		proc='
{
    "terminal": false,
    "execCPUAffinity": {
	    "initial": "'$cpus'"
    },
    "args": [ "/bin/true" ],
    "cwd": "/"
}'
		mask=$(cpus_to_mask "$cpus")
		echo "CPUS: $cpus, mask: $mask"
		run -0 runc --debug exec --process <(echo "$proc") ct1
		assert_output --regexp "nsexec.*: affinity: $mask"
	done
}

# Checks runc specific debug messages.
# bats test_tags=runc-specific
@test "runc exec [CPU affinity, initial and final set from process.json]" {
	first="$(first_cpu)"
	second=$((first + 1)) # Hacky; might not work in all environments.

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" ct1

	for cpus in "$second" "$first-$second" "$first,$second" "$first"; do
		proc='
{
    "terminal": false,
    "execCPUAffinity": {
	    "initial": "'$cpus'",
	    "final": "'$cpus'"
    },
    "args": [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ],
    "cwd": "/"
}'
		mask=$(cpus_to_mask "$cpus")
		exp=${cpus//,/-} # "," --> "-".
		echo "CPUS: $cpus, mask: $mask, final: $exp"
		run -0 runc --debug exec --process <(echo "$proc") ct1
		assert_output --regexp "nsexec.*: affinity: $mask"
		assert_output --partial "Cpus_allowed_list:	$exp" # Mind the literal tab.
	done
}

# Checks runc specific debug messages.
# bats test_tags=runc-specific
@test "runc exec [CPU affinity, initial and final set from config.json]" {
	initial="$(first_cpu)"
	final=$((initial + 1)) # Hacky; might not work in all environments.

	update_config "	  .process.execCPUAffinity.initial = \"$initial\"
			| .process.execCPUAffinity.final = \"$final\""

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" ct1

	run -0 runc --debug exec ct1 grep "Cpus_allowed_list:" /proc/self/status
	mask=$(cpus_to_mask "$initial")
	assert_output --regexp "nsexec.*: affinity: $mask"
	assert_output --partial "Cpus_allowed_list:	$final" # Mind the literal tab.
}

# Checks runc specific debug messages.
# bats test_tags=runc-specific
@test "runc exec [CPU affinity, cpuAffinity exec* set from config.json]" {
	initial="$(first_cpu)"
	final=$((initial + 1)) # Hacky; might not work in all environments.

	# Deprecated execCPUAffinity must be ignored when cpuAffinity is set.
	update_config "	  .process.execCPUAffinity.initial = \"$final\"
			| .process.execCPUAffinity.final = \"$initial\"
			| .process.cpuAffinity.execRuntime = \"$initial\"
			| .process.cpuAffinity.execContainer = \"$final\""

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" ct1

	run -0 runc --debug exec ct1 grep "Cpus_allowed_list:" /proc/self/status
	mask=$(cpus_to_mask "$initial")
	assert_output --regexp "nsexec.*: affinity: $mask"
	assert_output --partial "Cpus_allowed_list:	$final" # Mind the literal tab.
}

@test "runc exec [CPU affinity, execContainer=all]" {
	first="$(first_cpu)"

	update_config '.process.args = [ "/bin/sleep", "infinity" ]
		| .process.cpuAffinity.execRuntime = "'"$first"'"
		| .process.cpuAffinity.execContainer = "all"'
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	run -0 runc exec ct1 grep -F Cpus_allowed_list: /proc/self/status
	assert_output $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK"
}

@test "runc run [CPU affinity, createRuntime]" {
	first="$(first_cpu)"

	# createRuntime hook is run by runc on the host, so it inherits runc's affinity.
	# shellcheck disable=SC2016
	update_config --arg hook "grep -F Cpus_allowed_list: /proc/self/status > $(pwd)/hook.out" '
		.hooks |= {"createRuntime": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", $hook]}]}
		| .process.args = [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ]
		| .process.cpuAffinity.createRuntime = "'"$first"'"'
	run -0 runc run ctr
	# Container's process affinity should be reset.
	assert_output $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK"
	run -0 cat hook.out
	assert_output $'Cpus_allowed_list:\t'"$first"

	# Special value "all" resets runc's own affinity.
	update_config '.process.cpuAffinity.createRuntime = "all"'
	run -0 taskset -c "$first" runc run ctr
	run -0 cat hook.out
	assert_output $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK"
}

@test "runc run [CPU affinity, createContainer]" {
	first="$(first_cpu)"
	second=$((first + 1)) # Hacky; might not work in all environments.

	update_config '.process.args = [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ]
		| .process.cpuAffinity.createContainer = "'"$second"'"'
	run -0 taskset -c "$first" runc run ctr
	assert_output $'Cpus_allowed_list:\t'"$second"
}

@test "runc run [CPU affinity, startContainer]" {
	first="$(first_cpu)"
	second=$((first + 1)) # Hacky; might not work in all environments.

	# startContainer hook is run by runc init inside the container,
	# so it should have startContainer affinity, as well as the container
	# process. Also, check that startContainer works even if seccomp
	# (applied late due to noNewPrivileges) blocks sched_setaffinity.
	update_config '.root.readonly = false
		| .hooks |= {"startContainer": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", "grep -F Cpus_allowed_list: /proc/self/status > /hook.out"]}]}
		| .process.args = [ "/bin/sh", "-c", "cat /hook.out; grep -F Cpus_allowed_list: /proc/self/status" ]
		| .process.noNewPrivileges = true
		| .linux.seccomp = {
			"defaultAction": "SCMP_ACT_ALLOW",
			"syscalls": [{"names": ["sched_setaffinity"], "action": "SCMP_ACT_ERRNO"}]
		}
		| .process.cpuAffinity.createContainer = "'"$first"'"
		| .process.cpuAffinity.startContainer = "'"$second"'"'
	run -0 runc run ctr
	[ ${#lines[@]} -eq 2 ]
	[[ "${lines[0]}" == $'Cpus_allowed_list:\t'"$second" ]]
	[[ "${lines[1]}" == $'Cpus_allowed_list:\t'"$second" ]]
}

@test "runc run [CPU affinity should reset]" {
	first="$(first_cpu)"

	# Running without cpuset should result in an affinity for all CPUs.
	update_config '.process.args = [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ]'
	update_config 'del(.linux.resources.cpu)'
	run -0 taskset -c "$first" runc run ctr
	refute_output $'Cpus_allowed_list:\t'"$first"
	assert_output $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK"
}

@test "runc run [CPU affinity should reset to cgroup cpuset]" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	set_cgroups_path

	first="$(first_cpu)"
	second="$((first + 1))" # Hacky; might not work in all environments.

	# Running with a cpuset should result in an affinity that matches.
	update_config '.process.args = [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ]'
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$first-$second"'"}'
	run -0 taskset -c "$first" runc run ctr
	refute_output $'Cpus_allowed_list:\t'"$first"
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$first-$second" ]]

	# Ditto for a cpuset that has no overlap with the original cpumask.
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$second"'"}'
	run -0 taskset -c "$first" runc run ctr
	refute_output $'Cpus_allowed_list:\t'"$first"
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$second" ]]
}

@test "runc exec [default CPU affinity should reset]" {
	first="$(first_cpu)"

	# Running without cpuset should result in an affinity for all CPUs.
	update_config '.process.args = [ "/bin/sleep", "infinity" ]'
	update_config 'del(.linux.resources.cpu)'
	run -0 taskset -c "$first" runc run -d --console-socket "$CONSOLE_SOCKET" ctr3
	run -0 taskset -c "$first" runc exec ctr3 grep -F Cpus_allowed_list: /proc/self/status
	refute_output $'Cpus_allowed_list:\t'"$first"
	assert_output $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK"
}

@test "runc exec [default CPU affinity should reset to cgroup cpuset]" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	set_cgroups_path

	first="$(first_cpu)"
	second="$((first + 1))" # Hacky; might not work in all environments.

	# Running with a cpuset should result in an affinity that matches.
	update_config '.process.args = [ "/bin/sleep", "infinity" ]'
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$first-$second"'"}'
	run -0 taskset -c "$first" runc run -d --console-socket "$CONSOLE_SOCKET" ctr
	run -0 taskset -c "$first" runc exec ctr grep -F Cpus_allowed_list: /proc/self/status
	refute_output $'Cpus_allowed_list:\t'"$first"
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$first-$second" ]]

	# Stop the container so we can reconfigure it.
	run -0 runc delete -f ctr

	# Ditto for a cpuset that has no overlap with the original cpumask.
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$second"'"}'
	run -0 taskset -c "$first" runc run -d --console-socket "$CONSOLE_SOCKET" ctr
	run -0 taskset -c "$first" runc exec ctr grep -F Cpus_allowed_list: /proc/self/status
	refute_output $'Cpus_allowed_list:\t'"$first"
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$second" ]]
}
