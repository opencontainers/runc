#!/usr/bin/env bats

load helpers

function teardown() {
	rm -f "$BATS_RUN_TMPDIR"/runc-cgroups-integration-test.json
	teardown_bundle
}

function setup() {
	setup_busybox

	set_cgroups_path

	# Set some initial known values
	update_config ' .linux.resources.memory |= {"limit": 33554432, "reservation": 25165824}
			| .linux.resources.cpu |= {"shares": 100, "quota": 500000, "period": 1000000}
			| .linux.resources.pids |= {"limit": 20}'
}

# Tests whatever limits are (more or less) common between cgroup
# v1 and v2: memory/swap, pids, and cpuset.
@test "update cgroup v1/v2 common limits" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires cgroups_memory cgroups_pids cgroups_cpuset
	init_cgroup_paths

	# run a few busyboxes detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# Set a few variables to make the code below work for both v1 and v2
	if [ -v CGROUP_V1 ]; then
		MEM_LIMIT="memory.limit_in_bytes"
		SD_MEM_LIMIT="MemoryLimit"
		MEM_RESERVE="memory.soft_limit_in_bytes"
		SD_MEM_RESERVE="unsupported"
		MEM_SWAP="memory.memsw.limit_in_bytes"
		SD_MEM_SWAP="unsupported"
		SYSTEM_MEM=$(cat "${CGROUP_MEMORY_BASE_PATH}/${MEM_LIMIT}")
		HAVE_SWAP="no"
		if [ -f "${CGROUP_MEMORY_BASE_PATH}/${MEM_SWAP}" ]; then
			HAVE_SWAP="yes"
		fi
	else
		MEM_LIMIT="memory.max"
		SD_MEM_LIMIT="MemoryMax"
		MEM_RESERVE="memory.low"
		SD_MEM_RESERVE="MemoryLow"
		MEM_SWAP="memory.swap.max"
		SD_MEM_SWAP="MemorySwapMax"
		SYSTEM_MEM="max"
		HAVE_SWAP="yes"
	fi

	SD_UNLIMITED="infinity"
	SD_VERSION=$(systemctl --version | awk '{print $2; exit}')
	if [ "$SD_VERSION" -lt 227 ]; then
		SD_UNLIMITED="18446744073709551615"
	fi

	# check that initial values were properly set
	check_cgroup_value $MEM_LIMIT 33554432
	check_systemd_value $SD_MEM_LIMIT 33554432

	check_cgroup_value $MEM_RESERVE 25165824
	check_systemd_value $SD_MEM_RESERVE 25165824

	check_cgroup_value "pids.max" 20
	check_systemd_value "TasksMax" 20

	# update cpuset if possible (i.e. we're running on a multicore cpu)
	cpu_count=$(grep -c '^processor' /proc/cpuinfo)
	if [ "$cpu_count" -gt 1 ]; then
		runc update test_update --cpuset-cpus "1"
		[ "$status" -eq 0 ]
		check_cgroup_value "cpuset.cpus" 1
	fi

	# update memory limit
	runc update test_update --memory 67108864
	[ "$status" -eq 0 ]
	check_cgroup_value $MEM_LIMIT 67108864
	check_systemd_value $SD_MEM_LIMIT 67108864

	runc update test_update --memory 50M
	[ "$status" -eq 0 ]
	check_cgroup_value $MEM_LIMIT 52428800
	check_systemd_value $SD_MEM_LIMIT 52428800

	# update memory soft limit
	runc update test_update --memory-reservation 33554432
	[ "$status" -eq 0 ]
	check_cgroup_value "$MEM_RESERVE" 33554432
	check_systemd_value "$SD_MEM_RESERVE" 33554432

	# Run swap memory tests if swap is available
	if [ "$HAVE_SWAP" = "yes" ]; then
		# try to remove memory swap limit
		runc update test_update --memory-swap -1
		[ "$status" -eq 0 ]
		check_cgroup_value "$MEM_SWAP" "$SYSTEM_MEM"
		check_systemd_value "$SD_MEM_SWAP" "$SD_UNLIMITED"

		# update memory swap
		if [ -v CGROUP_V2 ]; then
			# for cgroupv2, memory and swap can only be set together
			runc update test_update --memory 52428800 --memory-swap 96468992
			[ "$status" -eq 0 ]
			# for cgroupv2, swap is a separate limit (it does not include mem)
			check_cgroup_value "$MEM_SWAP" $((96468992 - 52428800))
			check_systemd_value "$SD_MEM_SWAP" $((96468992 - 52428800))
		else
			runc update test_update --memory-swap 96468992
			[ "$status" -eq 0 ]
			check_cgroup_value "$MEM_SWAP" 96468992
			check_systemd_value "$SD_MEM_SWAP" 96468992
		fi
	fi

	# try to remove memory limit
	runc update test_update --memory -1
	[ "$status" -eq 0 ]

	# check memory limit is gone
	check_cgroup_value "$MEM_LIMIT" "$SYSTEM_MEM"
	check_systemd_value "$SD_MEM_LIMIT" "$SD_UNLIMITED"

	# check swap memory limited is gone
	if [ "$HAVE_SWAP" = "yes" ]; then
		check_cgroup_value "$MEM_SWAP" "$SYSTEM_MEM"
		check_systemd_value "$SD_MEM_SWAP" "$SD_UNLIMITED"
	fi

	# update pids limit
	runc update test_update --pids-limit 10
	[ "$status" -eq 0 ]
	check_cgroup_value "pids.max" 10
	check_systemd_value "TasksMax" 10

	# unlimited
	runc update test_update --pids-limit -1
	[ "$status" -eq 0 ]
	check_cgroup_value "pids.max" max
	check_systemd_value "TasksMax" $SD_UNLIMITED

	# Revert to the test initial value via json on stdin
	runc update -r - test_update <<EOF
{
  "memory": {
    "limit": 33554432,
    "reservation": 25165824
  },
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000,
    "cpus": "0"
  },
  "pids": {
    "limit": 20
  }
}
EOF
	[ "$status" -eq 0 ]
	check_cgroup_value "cpuset.cpus" 0

	check_cgroup_value $MEM_LIMIT 33554432
	check_systemd_value $SD_MEM_LIMIT 33554432

	check_cgroup_value $MEM_RESERVE 25165824
	check_systemd_value $SD_MEM_RESERVE 25165824

	check_cgroup_value "pids.max" 20
	check_systemd_value "TasksMax" 20

	# redo all the changes at once
	runc update test_update \
		--cpu-period 900000 --cpu-quota 600000 --cpu-share 200 \
		--memory 67108864 --memory-reservation 33554432 \
		--pids-limit 10
	[ "$status" -eq 0 ]
	check_cgroup_value $MEM_LIMIT 67108864
	check_systemd_value $SD_MEM_LIMIT 67108864

	check_cgroup_value $MEM_RESERVE 33554432
	check_systemd_value $SD_MEM_RESERVE 33554432

	check_cgroup_value "pids.max" 10
	check_systemd_value "TasksMax" 10

	# reset to initial test value via json file
	cat <<EOF >"$BATS_RUN_TMPDIR"/runc-cgroups-integration-test.json
{
  "memory": {
    "limit": 33554432,
    "reservation": 25165824
  },
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000,
    "cpus": "0"
  },
  "pids": {
    "limit": 20
  }
}
EOF

	runc update -r "$BATS_RUN_TMPDIR"/runc-cgroups-integration-test.json test_update
	[ "$status" -eq 0 ]
	check_cgroup_value "cpuset.cpus" 0

	check_cgroup_value $MEM_LIMIT 33554432
	check_systemd_value $SD_MEM_LIMIT 33554432

	check_cgroup_value $MEM_RESERVE 25165824
	check_systemd_value $SD_MEM_RESERVE 25165824

	check_cgroup_value "pids.max" 20
	check_systemd_value "TasksMax" 20

	if [ "$HAVE_SWAP" = "yes" ]; then
		# Test case for https://github.com/opencontainers/runc/pull/592,
		# checking libcontainer/cgroups/fs/memory.go:setMemoryAndSwap.

		runc update test_update --memory 30M --memory-swap 50M
		[ "$status" -eq 0 ]

		check_cgroup_value $MEM_LIMIT $((30 * 1024 * 1024))
		check_systemd_value $SD_MEM_LIMIT $((30 * 1024 * 1024))

		if [ -v CGROUP_V2 ]; then
			# for cgroupv2, swap does not include mem
			check_cgroup_value "$MEM_SWAP" $((20 * 1024 * 1024))
			check_systemd_value "$SD_MEM_SWAP" $((20 * 1024 * 1024))
		else
			check_cgroup_value "$MEM_SWAP" $((50 * 1024 * 1024))
			check_systemd_value "$SD_MEM_SWAP" $((50 * 1024 * 1024))
		fi

		# Now, set new memory to more than old swap
		runc update test_update --memory 60M --memory-swap 80M
		[ "$status" -eq 0 ]

		check_cgroup_value $MEM_LIMIT $((60 * 1024 * 1024))
		check_systemd_value $SD_MEM_LIMIT $((60 * 1024 * 1024))

		if [ -v CGROUP_V2 ]; then
			# for cgroupv2, swap does not include mem
			check_cgroup_value "$MEM_SWAP" $((20 * 1024 * 1024))
			check_systemd_value "$SD_MEM_SWAP" $((20 * 1024 * 1024))
		else
			check_cgroup_value "$MEM_SWAP" $((80 * 1024 * 1024))
			check_systemd_value "$SD_MEM_SWAP" $((80 * 1024 * 1024))
		fi
	fi
}

@test "update cgroup cpu limits" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	# run a few busyboxes detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# check that initial values were properly set
	check_cpu_quota 500000 1000000 "500ms"
	check_cpu_shares 100

	# update cpu period
	runc update test_update --cpu-period 900000
	[ "$status" -eq 0 ]
	check_cpu_quota 500000 900000 "560ms"

	# update cpu quota
	runc update test_update --cpu-quota 600000
	[ "$status" -eq 0 ]
	check_cpu_quota 600000 900000 "670ms"

	# remove cpu quota
	runc update test_update --cpu-quota -1
	[ "$status" -eq 0 ]
	check_cpu_quota -1 900000 "infinity"

	# update cpu-shares
	runc update test_update --cpu-share 200
	[ "$status" -eq 0 ]
	check_cpu_shares 200

	# Revert to the test initial value via json on stding
	runc update -r - test_update <<EOF
{
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000
  }
}
EOF
	[ "$status" -eq 0 ]
	check_cpu_quota 500000 1000000 "500ms"

	# redo all the changes at once
	runc update test_update \
		--cpu-period 900000 --cpu-quota 600000 --cpu-share 200
	[ "$status" -eq 0 ]
	check_cpu_quota 600000 900000 "670ms"
	check_cpu_shares 200

	# remove cpu quota and reset the period
	runc update test_update --cpu-quota -1 --cpu-period 100000
	[ "$status" -eq 0 ]
	check_cpu_quota -1 100000 "infinity"

	# reset to initial test value via json file
	cat <<EOF >"$BATS_RUN_TMPDIR"/runc-cgroups-integration-test.json
{
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000
  }
}
EOF
	[ "$status" -eq 0 ]

	runc update -r "$BATS_RUN_TMPDIR"/runc-cgroups-integration-test.json test_update
	[ "$status" -eq 0 ]
	check_cpu_quota 500000 1000000 "500ms"
	check_cpu_shares 100
}

@test "cpu burst" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires cgroups_cpu_burst

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]
	check_cpu_burst 0

	runc update test_update --cpu-period 900000 --cpu-burst 500000
	[ "$status" -eq 0 ]
	check_cpu_burst 500000

	runc update test_update --cpu-period 900000 --cpu-burst 0
	[ "$status" -eq 0 ]
	check_cpu_burst 0
}

@test "set cpu period with no quota" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	update_config '.linux.resources.cpu |= { "period": 1000000 }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	check_cpu_quota -1 1000000 "infinity"
}

@test "set cpu period with no quota (invalid period)" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	update_config '.linux.resources.cpu |= { "period": 100 }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 1 ]
}

@test "set cpu quota with no period" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	update_config '.linux.resources.cpu |= { "quota": 5000 }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]
	check_cpu_quota 5000 100000 "50ms"
}

@test "update cpu period with no previous period/quota set" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	update_config '.linux.resources.cpu |= {}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# update the period alone, no old values were set
	runc update --cpu-period 50000 test_update
	[ "$status" -eq 0 ]
	check_cpu_quota -1 50000 "infinity"
}

@test "update cpu quota with no previous period/quota set" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	update_config '.linux.resources.cpu |= {}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# update the quota alone, no old values were set
	runc update --cpu-quota 30000 test_update
	[ "$status" -eq 0 ]
	check_cpu_quota 30000 100000 "300ms"
}

@test "update cpu period in a pod cgroup with pod limit set" {
	requires cgroups_v1
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path "pod_${RANDOM}"

	# Set parent/pod CPU quota limit to 50%.
	if [ -v RUNC_USE_SYSTEMD ]; then
		set_parent_systemd_properties CPUQuota="50%"
	else
		echo 50000 >"/sys/fs/cgroup/cpu/$REL_PARENT_PATH/cpu.cfs_quota_us"
	fi
	# Sanity checks.
	run cat "/sys/fs/cgroup/cpu$REL_PARENT_PATH/cpu.cfs_period_us"
	[ "$output" -eq 100000 ]
	run cat "/sys/fs/cgroup/cpu$REL_PARENT_PATH/cpu.cfs_quota_us"
	[ "$output" -eq 50000 ]

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]
	# Get the current period.
	local cur
	cur=$(get_cgroup_value cpu.cfs_period_us)

	# Sanity check: as the parent cgroup sets the limit to 50%,
	# setting a higher limit (e.g. 60%) is expected to fail.
	runc update --cpu-quota $((cur * 6 / 10)) test_update
	[ "$status" -eq 1 ]

	# Finally, the test itself: set 30% limit but with lower period.
	runc update --cpu-period 10000 --cpu-quota 3000 test_update
	[ "$status" -eq 0 ]
	check_cpu_quota 3000 10000 "300ms"
}

@test "update cgroup cpu.idle" {
	requires cgroups_cpu_idle
	[ $EUID -ne 0 ] && requires rootless_cgroup

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	check_cgroup_value "cpu.idle" "0"

	local val
	for val in 1 0 1; do
		runc update -r - test_update <<EOF
{
  "cpu": {
    "idle": $val
  }
}
EOF
		[ "$status" -eq 0 ]
		check_cgroup_value "cpu.idle" "$val"
	done

	for val in 1 0 1; do
		runc update --cpu-idle "$val" test_update

		[ "$status" -eq 0 ]
		check_cgroup_value "cpu.idle" "$val"
	done

	# Values other than 1 or 0 are ignored by the kernel, see
	# sched_group_set_idle() in kernel/sched/fair.c.
	#
	# If this ever fails, it means that the kernel now accepts values
	# other than 0 or 1, and runc needs to adopt.
	for val in -1 2 3; do
		runc update --cpu-idle "$val" test_update
		[ "$status" -ne 0 ]
		check_cgroup_value "cpu.idle" "1"
	done

	# https://github.com/opencontainers/runc/issues/3786
	[ "$(systemd_version)" -ge 252 ] && return
	# test update other option won't impact on cpu.idle
	runc update --cpu-period 10000 test_update
	[ "$status" -eq 0 ]
	check_cgroup_value "cpu.idle" "1"
}

@test "update cgroup cpu.idle via systemd v252+" {
	requires cgroups_v2 systemd_v252 cgroups_cpu_idle
	[ $EUID -ne 0 ] && requires rootless_cgroup

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]
	check_cgroup_value "cpu.idle" "0"

	# If cpu-idle is set, cpu-share (converted to CPUWeight) can't be set via systemd.
	runc update --cpu-share 200 --cpu-idle 1 test_update
	[[ "$output" == *"unable to apply both"* ]]
	check_cgroup_value "cpu.idle" "1"

	# Changing cpu-shares (converted to CPU weight) resets cpu.idle to 0.
	runc update --cpu-share 200 test_update
	check_cgroup_value "cpu.idle" "0"

	# Setting values via unified map.

	# If cpu.idle is set, cpu.weight is ignored.
	runc update -r - test_update <<EOF
{
  "unified": {
    "cpu.idle": "1",
    "cpu.weight": "8"
  }
}
EOF
	[[ "$output" == *"unable to apply both"* ]]
	check_cgroup_value "cpu.idle" "1"

	# Setting any cpu.weight should reset cpu.idle to 0.
	runc update -r - test_update <<EOF
{
  "unified": {
    "cpu.weight": "8"
  }
}
EOF
	check_cgroup_value "cpu.idle" "0"
}

@test "update cgroup v2 resources via unified map" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires cgroups_v2

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# check that initial values were properly set
	check_cpu_quota 500000 1000000 "500ms"
	# initial cpu shares of 100 corresponds to weight of 4
	check_cpu_weight 4
	check_systemd_value "TasksMax" 20

	runc update -r - test_update <<EOF
{
  "unified": {
    "cpu.max": "max 100000",
    "cpu.weight": "16",
    "pids.max": "10"
  }
}
EOF

	# check the updated systemd unit properties
	check_cpu_quota -1 100000 "infinity"
	check_cpu_weight 16
	check_systemd_value "TasksMax" 10
}

@test "update cpuset parameters via resources.CPU" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires smp cgroups_cpuset

	local AllowedCPUs='AllowedCPUs' AllowedMemoryNodes='AllowedMemoryNodes'
	# these properties require systemd >= v244
	if [ "$(systemd_version)" -lt 244 ]; then
		# a hack to skip checks, see check_systemd_value()
		AllowedCPUs='unsupported'
		AllowedMemoryNodes='unsupported'
	fi

	update_config ' .linux.resources.CPU |= {
				"Cpus": "0",
				"Mems": "0"
			}'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# check that initial values were properly set
	check_systemd_value "$AllowedCPUs" 0
	check_systemd_value "$AllowedMemoryNodes" 0

	runc update -r - test_update <<EOF
{
  "CPU": {
    "Cpus": "1"
  }
}
EOF
	[ "$status" -eq 0 ]

	# check the updated systemd unit properties
	check_systemd_value "$AllowedCPUs" 1

	# More than 1 numa memory node is required to test this
	file="/sys/fs/cgroup/cpuset.mems.effective"
	if ! test -r $file || grep -q '^0$' $file; then
		# skip the rest of it
		return 0
	fi

	runc update -r - test_update <<EOF
{
  "CPU": {
    "Mems": "1"
  }
}
EOF
	[ "$status" -eq 0 ]

	# check the updated systemd unit properties
	check_systemd_value "$AllowedMemoryNodes" 1
}

@test "update cpuset parameters via v2 unified map" {
	# This test assumes systemd >= v244
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires cgroups_v2 smp cgroups_cpuset

	update_config ' .linux.resources.unified |= {
				"cpuset.cpus": "0",
				"cpuset.mems": "0"
			}'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# check that initial values were properly set
	check_systemd_value "AllowedCPUs" 0
	check_systemd_value "AllowedMemoryNodes" 0

	runc update -r - test_update <<EOF
{
  "unified": {
    "cpuset.cpus": "1"
  }
}
EOF
	[ "$status" -eq 0 ]

	# check the updated systemd unit properties
	check_systemd_value "AllowedCPUs" 1

	# More than 1 numa memory node is required to test this
	file="/sys/fs/cgroup/cpuset.mems.effective"
	if ! test -r $file || grep -q '^0$' $file; then
		# skip the rest of it
		return 0
	fi

	runc update -r - test_update <<EOF
{
  "unified": {
    "cpuset.mems": "1"
  }
}
EOF
	[ "$status" -eq 0 ]

	# check the updated systemd unit properties
	check_systemd_value "AllowedMemoryNodes" 1
}

@test "update cpuset cpus range via v2 unified map" {
	# This test assumes systemd >= v244
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires systemd cgroups_v2 more_than_8_core cgroups_cpuset

	update_config ' .linux.resources.unified |= {
				"cpuset.cpus": "0-5",
			}'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# check that the initial value was properly set
	check_systemd_value "AllowedCPUs" "0-5"

	runc update -r - test_update <<EOF
{
  "unified": {
    "cpuset.cpus": "5-8"
  }
}
EOF
	[ "$status" -eq 0 ]

	# check the updated systemd unit property, the value should not be affected by byte order
	check_systemd_value "AllowedCPUs" "5-8"
}

@test "update rt period and runtime" {
	[ $EUID -ne 0 ] && requires rootless_cgroup
	requires cgroups_v1 cgroups_rt no_systemd

	local cgroup_cpu="${CGROUP_CPU_BASE_PATH}/${REL_CGROUPS_PATH}"

	# By default, "${cgroup_cpu}/cpu.rt_runtime_us" is set to 0, which inhibits
	# setting the container's realtimeRuntime. (#2046)
	#
	# When ${cgroup_cpu} is "/sys/fs/cgroup/cpu,cpuacct/runc-cgroups-integration-test/test-cgroup",
	# we write the values of /sys/fs/cgroup/cpu,cpuacct/cpu.rt_{period,runtime}_us to:
	# - sys/fs/cgroup/cpu,cpuacct/runc-cgroups-integration-test/cpu.rt_{period,runtime}_us
	# - sys/fs/cgroup/cpu,cpuacct/runc-cgroups-integration-test/test-cgroup/cpu.rt_{period,runtime}_us
	#
	# Typically period=1000000 runtime=950000 .
	#
	# TODO: support systemd
	mkdir -p "$cgroup_cpu"
	local root_period root_runtime
	root_period=$(cat "${CGROUP_CPU_BASE_PATH}/cpu.rt_period_us")
	root_runtime=$(cat "${CGROUP_CPU_BASE_PATH}/cpu.rt_runtime_us")
	# the following IFS magic sets dirs=("runc-cgroups-integration-test" "test-cgroup")
	IFS='/' read -r -a dirs <<<"${REL_CGROUPS_PATH#/}"
	for ((i = 0; i < ${#dirs[@]}; i++)); do
		local target="$CGROUP_CPU_BASE_PATH"
		for ((j = 0; j <= i; j++)); do
			target="${target}/${dirs[$j]}"
		done
		target_period="${target}/cpu.rt_period_us"
		echo "Writing ${root_period} to ${target_period}"
		echo "$root_period" >"$target_period"
		target_runtime="${target}/cpu.rt_runtime_us"
		echo "Writing ${root_runtime} to ${target_runtime}"
		echo "$root_runtime" >"$target_runtime"
	done

	# run a detached busybox
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update_rt
	[ "$status" -eq 0 ]

	runc update -r - test_update_rt <<EOF
{
  "cpu": {
    "realtimeRuntime": 500001
  }
}
EOF
	[ "$status" -eq 0 ]
	check_cgroup_value "cpu.rt_period_us" "$root_period"
	check_cgroup_value "cpu.rt_runtime_us" 500001

	runc update -r - test_update_rt <<EOF
{
  "cpu": {
    "realtimePeriod": 800001,
    "realtimeRuntime": 500001
  }
}
EOF
	check_cgroup_value "cpu.rt_period_us" 800001
	check_cgroup_value "cpu.rt_runtime_us" 500001

	runc update test_update_rt --cpu-rt-period 900001 --cpu-rt-runtime 600001
	[ "$status" -eq 0 ]

	check_cgroup_value "cpu.rt_period_us" 900001
	check_cgroup_value "cpu.rt_runtime_us" 600001
}

@test "update devices [minimal transition rules]" {
	requires root

	# Run a basic shell script that tries to read from /dev/kmsg, but
	# due to lack of permissions, it prints the error message to /dev/null.
	# If any data is read from /dev/kmsg, it will be printed to stdout, and the
	# test will fail. In the same way, if access to /dev/null is denied, the
	# error will be printed to stderr, and the test will also fail.
	#
	# "runc update" makes use of minimal transition rules, updates should not cause
	# writes to fail at any point. For systemd cgroup driver on cgroup v1, the cgroup
	# is frozen to ensure this.
	update_config ' .linux.resources.devices = [{"allow": false, "access": "rwm"}, {"allow": false, "type": "c", "major": 1, "minor": 11, "access": "rwa"}]
			| .linux.devices = [{"path": "/dev/kmsg", "type": "c", "major": 1, "minor": 11}]
			| .process.capabilities.bounding += ["CAP_SYSLOG"]
			| .process.capabilities.effective += ["CAP_SYSLOG"]
			| .process.capabilities.permitted += ["CAP_SYSLOG"]
			| .process.args |= ["sh", "-c", "while true; do head -c 100 /dev/kmsg 2> /dev/null; done"]'

	# Set up a temporary console socket and recvtty so we can get the stdio.
	TMP_RECVTTY_DIR="$(mktemp -d "$BATS_RUN_TMPDIR/runc-tmp-recvtty.XXXXXX")"
	TMP_RECVTTY_PID="$TMP_RECVTTY_DIR/recvtty.pid"
	TMP_CONSOLE_SOCKET="$TMP_RECVTTY_DIR/console.sock"
	CONTAINER_OUTPUT="$TMP_RECVTTY_DIR/output"
	("$RECVTTY" --no-stdin --pid-file "$TMP_RECVTTY_PID" \
		--mode single "$TMP_CONSOLE_SOCKET" &>"$CONTAINER_OUTPUT") &
	retry 10 0.1 [ -e "$TMP_CONSOLE_SOCKET" ]

	# Run the container in the background.
	runc run -d --console-socket "$TMP_CONSOLE_SOCKET" test_update
	cat "$CONTAINER_OUTPUT"
	[ "$status" -eq 0 ]

	# Trigger an update. This update doesn't actually change the device rules,
	# but it will trigger the devices cgroup code to reapply the current rules.
	# We trigger the update a few times to make sure we hit the race.
	for _ in {1..30}; do
		# TODO: Update "runc update" so we can change the device rules.
		runc update --pids-limit 30 test_update
		[ "$status" -eq 0 ]
	done

	# Kill recvtty.
	kill -9 "$(<"$TMP_RECVTTY_PID")"

	# There should've been no output from the container.
	cat "$CONTAINER_OUTPUT"
	[ -z "$(<"$CONTAINER_OUTPUT")" ]
}

@test "update paused container" {
	requires cgroups_freezer
	[ $EUID -ne 0 ] && requires rootless_cgroup

	# Run the container in the background.
	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# Pause the container.
	runc pause test_update
	[ "$status" -eq 0 ]

	# Trigger an unrelated update.
	runc update --pids-limit 30 test_update
	[ "$status" -eq 0 ]

	# The container should still be paused.
	testcontainer test_update paused

	# Resume the container.
	runc resume test_update
	[ "$status" -eq 0 ]
}

@test "update memory vs CheckBeforeUpdate" {
	requires cgroups_v2
	[ $EUID -ne 0 ] && requires rootless_cgroup

	runc run -d --console-socket "$CONSOLE_SOCKET" test_update
	[ "$status" -eq 0 ]

	# Setting memory to low value with checkBeforeUpdate=true should fail.
	runc update -r - test_update <<EOF
{
  "memory": {
    "limit": 1024,
    "checkBeforeUpdate": true
  }
}
EOF
	[ "$status" -ne 0 ]
	[[ "$output" == *"rejecting memory limit"* ]]
	testcontainer test_update running

	# Setting memory+swap to low value with checkBeforeUpdate=true should fail.
	runc update -r - test_update <<EOF
{
  "memory": {
    "limit": 1024,
    "swap": 2048,
    "checkBeforeUpdate": true
  }
}
EOF
	[ "$status" -ne 0 ]
	[[ "$output" == *"rejecting memory+swap limit"* ]]
	testcontainer test_update running

	# The container will be OOM killed, and runc might either succeed
	# or fail depending on the timing, so we don't check its exit code.
	runc update test_update --memory 1024
	wait_for_container 10 1 test_update stopped
}
