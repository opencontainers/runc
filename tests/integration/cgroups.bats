#!/usr/bin/env bats

load helpers

function teardown() {
	teardown_running_container test_cgroups_kmem
	teardown_running_container test_cgroups_permissions
	teardown_running_container test_cgroups_group
	teardown_running_container test_cgroups_unified
	teardown_busybox
}

function setup() {
	teardown
	setup_busybox
}

@test "runc update --kernel-memory{,-tcp} (initialized)" {
	[[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
	requires cgroups_kmem

	set_cgroups_path "$BUSYBOX_BUNDLE"

	# Set some initial known values
	update_config '.linux.resources.memory |= {"kernel": 16777216, "kernelTCP": 11534336}' "${BUSYBOX_BUNDLE}"

	# run a detached busybox to work with
	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_kmem
	[ "$status" -eq 0 ]

	check_cgroup_value "memory.kmem.limit_in_bytes" 16777216
	check_cgroup_value "memory.kmem.tcp.limit_in_bytes" 11534336

	# update kernel memory limit
	runc update test_cgroups_kmem --kernel-memory 50331648
	[ "$status" -eq 0 ]
	check_cgroup_value "memory.kmem.limit_in_bytes" 50331648

	# update kernel memory tcp limit
	runc update test_cgroups_kmem --kernel-memory-tcp 41943040
	[ "$status" -eq 0 ]
	check_cgroup_value "memory.kmem.tcp.limit_in_bytes" 41943040
}

@test "runc update --kernel-memory (uninitialized)" {
	[[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
	requires cgroups_kmem

	set_cgroups_path "$BUSYBOX_BUNDLE"

	# run a detached busybox to work with
	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_kmem
	[ "$status" -eq 0 ]

	# update kernel memory limit
	runc update test_cgroups_kmem --kernel-memory 50331648
	# Since kernel 4.6, we can update kernel memory without initialization
	# because it's accounted by default.
	if [[ "$KERNEL_MAJOR" -lt 4 || ("$KERNEL_MAJOR" -eq 4 && "$KERNEL_MINOR" -le 5) ]]; then
		[ ! "$status" -eq 0 ]
	else
		[ "$status" -eq 0 ]
		check_cgroup_value "memory.kmem.limit_in_bytes" 50331648
	fi
}

@test "runc create (no limits + no cgrouppath + no permission) succeeds" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 0 ]
}

@test "runc create (rootless + no limits + cgrouppath + no permission) fails with permission error" {
	requires rootless
	requires rootless_no_cgroup
	# systemd controls the permission, so error does not happen
	requires no_systemd

	set_cgroups_path "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 1 ]
	[[ ${lines[0]} == *"permission denied"* ]]
}

@test "runc create (rootless + limits + no cgrouppath + no permission) fails with informative error" {
	requires rootless
	requires rootless_no_cgroup
	# systemd controls the permission, so error does not happen
	requires no_systemd

	set_resources_limit "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 1 ]
	[[ ${lines[0]} == *"rootless needs no limits + no cgrouppath when no permission is granted for cgroups"* ]] || [[ ${lines[0]} == *"cannot set pids limit: container could not join or create cgroup"* ]]
}

@test "runc create (limits + cgrouppath + permission on the cgroup dir) succeeds" {
	[[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

	set_cgroups_path "$BUSYBOX_BUNDLE"
	set_resources_limit "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 0 ]
	if [ "$CGROUP_UNIFIED" != "no" ]; then
		if [ -n "${RUNC_USE_SYSTEMD}" ]; then
			if [ "$(id -u)" = "0" ]; then
				check_cgroup_value "cgroup.controllers" "$(cat /sys/fs/cgroup/machine.slice/cgroup.controllers)"
			else
				# shellcheck disable=SC2046
				check_cgroup_value "cgroup.controllers" "$(cat /sys/fs/cgroup/user.slice/user-$(id -u).slice/cgroup.controllers)"
			fi
		else
			check_cgroup_value "cgroup.controllers" "$(cat /sys/fs/cgroup/cgroup.controllers)"
		fi
	fi
}

@test "runc exec (limits + cgrouppath + permission on the cgroup dir) succeeds" {
	[[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

	set_cgroups_path "$BUSYBOX_BUNDLE"
	set_resources_limit "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 0 ]

	runc exec test_cgroups_permissions echo "cgroups_exec"
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == *"cgroups_exec"* ]]
}

@test "runc exec (cgroup v2 + init process in non-root cgroup) succeeds" {
	requires root cgroups_v2

	set_cgroups_path "$BUSYBOX_BUNDLE"
	set_cgroup_mount_writable "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_group
	[ "$status" -eq 0 ]

	runc exec test_cgroups_group cat /sys/fs/cgroup/cgroup.controllers
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == *"memory"* ]]

	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == "0::/" ]]

	runc exec test_cgroups_group mkdir /sys/fs/cgroup/foo
	[ "$status" -eq 0 ]

	runc exec test_cgroups_group sh -c "echo 1 > /sys/fs/cgroup/foo/cgroup.procs"
	[ "$status" -eq 0 ]

	# the init process is now in "/foo", but an exec process can still join "/"
	# because we haven't enabled any domain controller.
	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == "0::/" ]]

	# turn on a domain controller (memory)
	runc exec test_cgroups_group sh -euxc 'echo $$ > /sys/fs/cgroup/foo/cgroup.procs; echo +memory > /sys/fs/cgroup/cgroup.subtree_control'
	[ "$status" -eq 0 ]

	# an exec process can no longer join "/" after turning on a domain controller.
	# falls back to "/foo".
	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == "0::/foo" ]]

	# teardown: remove "/foo"
	# shellcheck disable=SC2016
	runc exec test_cgroups_group sh -uxc 'echo -memory > /sys/fs/cgroup/cgroup.subtree_control; for f in $(cat /sys/fs/cgroup/foo/cgroup.procs); do echo $f > /sys/fs/cgroup/cgroup.procs; done; rmdir /sys/fs/cgroup/foo'
	runc exec test_cgroups_group test ! -d /sys/fs/cgroup/foo
	[ "$status" -eq 0 ]
	#
}

@test "runc run (cgroup v1 + unified resources should fail)" {
	requires root cgroups_v1

	set_cgroups_path "$BUSYBOX_BUNDLE"
	set_resources_limit "$BUSYBOX_BUNDLE"
	update_config '.linux.resources.unified |= {"memory.min": "131072"}' "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -ne 0 ]
	[[ "$output" == *'invalid configuration'* ]]
}

@test "runc run (cgroup v2 resources.unified only)" {
	requires root cgroups_v2

	set_cgroups_path "$BUSYBOX_BUNDLE"
	update_config ' .linux.resources.unified |= {
				"memory.min":   "131072",
				"memory.low":   "524288",
				"memory.high": "5242880",
				"memory.max": "10485760",
				"memory.swap.max": "20971520",
				"pids.max": "99",
				"cpu.max": "10000 100000",
				"cpu.weight": "42"
			}' "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]

	runc exec test_cgroups_unified sh -c 'cd /sys/fs/cgroup && grep . *.min *.max *.low *.high'
	[ "$status" -eq 0 ]
	echo "$output"

	echo "$output" | grep -q '^memory.min:131072$'
	echo "$output" | grep -q '^memory.low:524288$'
	echo "$output" | grep -q '^memory.high:5242880$'
	echo "$output" | grep -q '^memory.max:10485760$'
	echo "$output" | grep -q '^memory.swap.max:20971520$'
	echo "$output" | grep -q '^pids.max:99$'
	echo "$output" | grep -q '^cpu.max:10000 100000$'

	check_systemd_value "MemoryMin" 131072
	check_systemd_value "MemoryLow" 524288
	check_systemd_value "MemoryHigh" 5242880
	check_systemd_value "MemoryMax" 10485760
	check_systemd_value "MemorySwapMax" 20971520
	check_systemd_value "TasksMax" 99
	check_cpu_quota 10000 100000 "100ms"
	check_cpu_weight 42
}

@test "runc run (cgroup v2 resources.unified override)" {
	requires root cgroups_v2

	set_cgroups_path "$BUSYBOX_BUNDLE"
	# CPU shares of 3333 corresponds to CPU weight of 128.
	update_config '   .linux.resources.memory |= {"limit": 33554432}
			| .linux.resources.memorySwap |= {"limit": 33554432}
			| .linux.resources.cpu |= {
				"shares": 3333,
				"quota": 40000,
				"period": 100000
			}
			| .linux.resources.unified |= {
				"memory.min": "131072",
				"memory.max": "10485760",
				"pids.max": "42",
				"cpu.max": "5000 50000",
				"cpu.weight": "42"
			}' "$BUSYBOX_BUNDLE"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]

	runc exec test_cgroups_unified cat /sys/fs/cgroup/memory.min
	[ "$status" -eq 0 ]
	[ "$output" = '131072' ]

	runc exec test_cgroups_unified cat /sys/fs/cgroup/memory.max
	[ "$status" -eq 0 ]
	[ "$output" = '10485760' ]

	runc exec test_cgroups_unified cat /sys/fs/cgroup/pids.max
	[ "$status" -eq 0 ]
	[ "$output" = '42' ]
	check_systemd_value "TasksMax" 42

	check_cpu_quota 5000 50000 "100ms"

	check_cpu_weight 42
}
