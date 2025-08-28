#!/usr/bin/env bats
# Exec CPU affinity tests. For more details, see:
#  - https://github.com/opencontainers/runtime-spec/pull/1253

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

@test "runc exec [CPU affinity, only initial set from process.json]" {
	first="$(first_cpu)"
	second=$((first + 1)) # Hacky; might not work in all environments.

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

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
		runc --debug exec --process <(echo "$proc") ct1
		[[ "$output" == *"nsexec"*": affinity: $mask"* ]]
	done
}

@test "runc exec [CPU affinity, initial and final set from process.json]" {
	first="$(first_cpu)"
	second=$((first + 1)) # Hacky; might not work in all environments.

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

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
		runc --debug exec --process <(echo "$proc") ct1
		[[ "$output" == *"nsexec"*": affinity: $mask"* ]]
		[[ "$output" == *"Cpus_allowed_list:	$exp"* ]] # Mind the literal tab.
	done
}

@test "runc exec [CPU affinity, initial and final set from config.json]" {
	initial="$(first_cpu)"
	final=$((initial + 1)) # Hacky; might not work in all environments.

	update_config "	  .process.execCPUAffinity.initial = \"$initial\"
			| .process.execCPUAffinity.final = \"$final\""

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

	runc --debug exec ct1 grep "Cpus_allowed_list:" /proc/self/status
	[ "$status" -eq 0 ]
	mask=$(cpus_to_mask "$initial")
	[[ "$output" == *"nsexec"*": affinity: $mask"* ]]
	[[ "$output" == *"Cpus_allowed_list:	$final"* ]] # Mind the literal tab.
}

@test "runc run [CPU affinity should reset]" {
	# We need to use RUNC_CMDLINE since taskset requires a proper binary, not a
	# bash function (which is what runc and __runc are).
	setup_runc_cmdline

	first="$(first_cpu)"

	# Running without cpuset should result in an affinity for all CPUs.
	update_config '.process.args = [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ]'
	update_config 'del(.linux.resources.cpu)'
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" run ctr
	[ "$status" -eq 0 ]
	[[ "$output" != $'Cpus_allowed_list:\t'"$first" ]]
	[[ "$output" == $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK" ]]
}

@test "runc run [CPU affinity should reset to cgroup cpuset]" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	set_cgroups_path

	# We need to use RUNC_CMDLINE since taskset requires a proper binary, not a
	# bash function (which is what runc and __runc are).
	setup_runc_cmdline

	first="$(first_cpu)"
	second="$((first + 1))" # Hacky; might not work in all environments.

	# Running with a cpuset should result in an affinity that matches.
	update_config '.process.args = [ "/bin/grep", "-F", "Cpus_allowed_list:", "/proc/self/status" ]'
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$first-$second"'"}'
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" run ctr
	[ "$status" -eq 0 ]
	[[ "$output" != $'Cpus_allowed_list:\t'"$first" ]]
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$first-$second" ]]

	# Ditto for a cpuset that has no overlap with the original cpumask.
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$second"'"}'
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" run ctr
	[ "$status" -eq 0 ]
	[[ "$output" != $'Cpus_allowed_list:\t'"$first" ]]
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$second" ]]
}

@test "runc exec [default CPU affinity should reset]" {
	# We need to use RUNC_CMDLINE since taskset requires a proper binary, not a
	# bash function (which is what runc and __runc are).
	setup_runc_cmdline

	first="$(first_cpu)"

	# Running without cpuset should result in an affinity for all CPUs.
	update_config '.process.args = [ "/bin/sleep", "infinity" ]'
	update_config 'del(.linux.resources.cpu)'
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" run -d --console-socket "$CONSOLE_SOCKET" ctr3
	[ "$status" -eq 0 ]
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" exec ctr3 grep -F Cpus_allowed_list: /proc/self/status
	[ "$status" -eq 0 ]
	[[ "$output" != $'Cpus_allowed_list:\t'"$first" ]]
	[[ "$output" == $'Cpus_allowed_list:\t'"$INITIAL_CPU_MASK" ]]
}

@test "runc exec [default CPU affinity should reset to cgroup cpuset]" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	set_cgroups_path

	# We need to use RUNC_CMDLINE since taskset requires a proper binary, not a
	# bash function (which is what runc and __runc are).
	setup_runc_cmdline

	first="$(first_cpu)"
	second="$((first + 1))" # Hacky; might not work in all environments.

	# Running with a cpuset should result in an affinity that matches.
	update_config '.process.args = [ "/bin/sleep", "infinity" ]'
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$first-$second"'"}'
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" run -d --console-socket "$CONSOLE_SOCKET" ctr
	[ "$status" -eq 0 ]
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" exec ctr grep -F Cpus_allowed_list: /proc/self/status
	[ "$status" -eq 0 ]
	[[ "$output" != $'Cpus_allowed_list:\t'"$first" ]]
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$first-$second" ]]

	# Stop the container so we can reconfigure it.
	runc delete -f ctr
	[ "$status" -eq 0 ]

	# Ditto for a cpuset that has no overlap with the original cpumask.
	update_config '.linux.resources.cpu = {"mems": "0", "cpus": "'"$second"'"}'
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" run -d --console-socket "$CONSOLE_SOCKET" ctr
	[ "$status" -eq 0 ]
	sane_run taskset -c "$first" "${RUNC_CMDLINE[@]}" exec ctr grep -F Cpus_allowed_list: /proc/self/status
	[ "$status" -eq 0 ]
	[[ "$output" != $'Cpus_allowed_list:\t'"$first" ]]
	# XXX: For some reason, systemd-cgroup leads to us using the all-set
	#      cpumask rather than the cpuset we configured?
	[ -v RUNC_USE_SYSTEMD ] || [[ "$output" == $'Cpus_allowed_list:\t'"$second" ]]
}
