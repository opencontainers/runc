#!/usr/bin/env bats

load helpers

function teardown() {
	teardown_bundle
	teardown_loopdevs
}

function setup() {
	setup_busybox
}

@test "runc create (no limits + no cgrouppath + no permission) succeeds" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 0 ]
}

@test "runc create (rootless + no limits + cgrouppath + no permission) fails with permission error" {
	requires rootless rootless_no_cgroup

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 1 ]
	[[ "$output" == *"unable to apply cgroup configuration"*"permission denied"* ]]
}

@test "runc create (rootless + limits + no cgrouppath + no permission) fails with informative error" {
	requires rootless rootless_no_cgroup

	set_resources_limit

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 1 ]
	[[ "$output" == *"rootless needs no limits + no cgrouppath when no permission is granted for cgroups"* ]] ||
		[[ "$output" == *"cannot set pids limit: container could not join or create cgroup"* ]]
}

@test "runc create (limits + cgrouppath + permission on the cgroup dir) succeeds" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path
	set_resources_limit

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 0 ]
	if [ -v CGROUP_V2 ]; then
		if [ -v RUNC_USE_SYSTEMD ]; then
			if [ $EUID -eq 0 ]; then
				check_cgroup_value "cgroup.controllers" "$(cat /sys/fs/cgroup/machine.slice/cgroup.controllers)"
			else
				# Filter out controllers that systemd is unable to delegate.
				check_cgroup_value "cgroup.controllers" "$(sed 's/ \(dmem\|hugetlb\|misc\|rdma\)//g' </sys/fs/cgroup/user.slice/user-${EUID}.slice/cgroup.controllers)"
			fi
		else
			check_cgroup_value "cgroup.controllers" "$(cat /sys/fs/cgroup/cgroup.controllers)"
		fi
	fi
}

@test "runc exec (limits + cgrouppath + permission on the cgroup dir) succeeds" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path
	set_resources_limit

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_permissions
	[ "$status" -eq 0 ]

	runc exec test_cgroups_permissions echo "cgroups_exec"
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == *"cgroups_exec"* ]]
}

@test "runc exec (cgroup v2 + init process in non-root cgroup) succeeds" {
	requires root cgroups_v2

	set_cgroups_path
	set_cgroup_mount_writable

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_group
	[ "$status" -eq 0 ]

	runc exec test_cgroups_group cat /sys/fs/cgroup/cgroup.controllers
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == *"memory"* ]]

	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ ${lines[0]} = "0::/" ]]

	runc exec test_cgroups_group mkdir /sys/fs/cgroup/foo
	[ "$status" -eq 0 ]

	runc exec test_cgroups_group sh -c "echo 1 > /sys/fs/cgroup/foo/cgroup.procs"
	[ "$status" -eq 0 ]

	# the init process is now in "/foo", but an exec process can still join "/"
	# because we haven't enabled any domain controller.
	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ ${lines[0]} = "0::/" ]]

	# turn on a domain controller (memory)
	runc exec test_cgroups_group sh -euxc 'echo $$ > /sys/fs/cgroup/foo/cgroup.procs; echo +memory > /sys/fs/cgroup/cgroup.subtree_control'
	[ "$status" -eq 0 ]

	# an exec process can no longer join "/" after turning on a domain controller.
	# falls back to "/foo".
	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ ${lines[0]} = "0::/foo" ]]

	# teardown: remove "/foo"
	# shellcheck disable=SC2016
	runc exec test_cgroups_group sh -uxc 'echo -memory > /sys/fs/cgroup/cgroup.subtree_control; for f in $(cat /sys/fs/cgroup/foo/cgroup.procs); do echo $f > /sys/fs/cgroup/cgroup.procs; done; rmdir /sys/fs/cgroup/foo'
	runc exec test_cgroups_group test ! -d /sys/fs/cgroup/foo
	[ "$status" -eq 0 ]
	#
}

@test "runc run (cgroup v1 + unified resources should fail)" {
	requires root cgroups_v1

	set_cgroups_path
	set_resources_limit
	update_config '.linux.resources.unified |= {"memory.min": "131072"}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -ne 0 ]
	[[ "$output" == *'invalid configuration'* ]]
}

@test "runc run (blkio weight)" {
	requires cgroups_v2 cgroups_io_weight
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path
	update_config '.linux.resources.blockIO |= {"weight": 750}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]

	runc exec test_cgroups_unified sh -c 'cat /sys/fs/cgroup/io.bfq.weight'
	if [[ "$status" -eq 0 ]]; then
		[ "$output" = 'default 750' ]
	else
		runc exec test_cgroups_unified sh -c 'cat /sys/fs/cgroup/io.weight'
		[ "$output" = 'default 7475' ]
	fi
}

@test "runc run (per-device io weight for bfq)" {
	requires root # to create a loop device

	dev="$(setup_loopdev)"

	# Some distributions (like openSUSE) have udev configured to forcefully
	# reset the scheduler for loopN devices to be "none", which breaks this
	# test. We cannot modify the udev behaviour of the host, but since this is
	# usually triggered by the "change" event from losetup, we can wait for a
	# little bit before continuing the test. For more details, see
	# <https://github.com/opencontainers/runc/issues/4781>.
	sleep 2s

	# See if BFQ scheduler is available.
	if ! { grep -qw bfq "/sys/block/${dev#/dev/}/queue/scheduler" &&
		echo bfq >"/sys/block/${dev#/dev/}/queue/scheduler"; }; then
		skip "BFQ scheduler not available"
	fi

	# Check that the device still has the right scheduler, in case we lost the
	# race above...
	if ! grep -qw '\[bfq\]' "/sys/block/${dev#/dev/}/queue/scheduler"; then
		skip "udev is configured to reset loop device io scheduler"
	fi

	set_cgroups_path

	IFS=$' \t:' read -r major minor <<<"$(lsblk -nd -o MAJ:MIN "$dev")"
	update_config '	  .linux.devices += [{path: "'"$dev"'", type: "b", major: '"$major"', minor: '"$minor"'}]
			| .linux.resources.blockIO.weight |= 333
			| .linux.resources.blockIO.weightDevice |= [
				{ major: '"$major"', minor: '"$minor"', weight: 444 }
			]'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_dev_weight
	[ "$status" -eq 0 ]

	if [ -v CGROUP_V2 ]; then
		file="io.bfq.weight"
	else
		file="blkio.bfq.weight_device"
	fi
	weights1=$(get_cgroup_value $file)

	# Check that runc update works.
	runc update -r - test_dev_weight <<EOF
{
  "blockIO": {
    "weight": 111,
    "weightDevice": [
      {
        "major": $major,
        "minor": $minor,
        "weight": 222
      }
    ]
  }
}
EOF
	weights2=$(get_cgroup_value $file)

	# Check original values.
	grep '^default 333$' <<<"$weights1"
	grep "^$major:$minor 444$" <<<"$weights1"
	# Check updated values.
	grep '^default 111$' <<<"$weights2"
	grep "^$major:$minor 222$" <<<"$weights2"

}

@test "runc run (per-device multiple iops via unified)" {
	requires root cgroups_v2

	dev1="$(setup_loopdev)"
	dev2="$(setup_loopdev)"

	set_cgroups_path

	IFS=$' \t:' read -r major1 minor1 <<<"$(lsblk -nd -o MAJ:MIN "$dev1")"
	IFS=$' \t:' read -r major2 minor2 <<<"$(lsblk -nd -o MAJ:MIN "$dev2")"
	update_config '	  .linux.devices += [
				{path: "'"$dev1"'", type: "b", major: '"$major1"', minor: '"$minor1"'},
				{path: "'"$dev2"'", type: "b", major: '"$major2"', minor: '"$minor2"'}
			   ]
			| .linux.resources.unified |=
				{"io.max": "'"$major1"':'"$minor1"' riops=333 wiops=444\n'"$major2"':'"$minor2"' riops=555 wiops=666\n"}'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_dev_weight
	[ "$status" -eq 0 ]

	weights=$(get_cgroup_value "io.max")
	grep "^$major1:$minor1 .* riops=333 wiops=444$" <<<"$weights"
	grep "^$major2:$minor2 .* riops=555 wiops=666$" <<<"$weights"
}

@test "runc run (cpu.idle)" {
	requires cgroups_cpu_idle
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path
	update_config '.linux.resources.cpu.idle = 1'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]
	check_cgroup_value "cpu.idle" "1"
}

# Convert size in KB to hugetlb size suffix.
convert_hugetlb_size() {
	local size=$1
	local units=("KB" "MB" "GB")
	local idx=0

	while ((size >= 1024)); do
		((size /= 1024))
		((idx++))
	done

	echo "$size${units[$idx]}"
}

@test "runc run (hugetlb limits)" {
	requires cgroups_hugetlb
	[ $EUID -ne 0 ] && requires rootless_cgroup
	# shellcheck disable=SC2012 # ls is fine here.
	mapfile -t sizes_kb < <(ls /sys/kernel/mm/hugepages/ | sed -e 's/.*hugepages-//' -e 's/kB$//') #
	if [ "${#sizes_kb[@]}" -lt 1 ]; then
		skip "requires hugetlb"
	fi

	# Create two arrays:
	#  - sizes: hugetlb cgroup file suffixes;
	#  - limits: limits for each size.
	for size in "${sizes_kb[@]}"; do
		sizes+=("$(convert_hugetlb_size "$size")")
		# Limit to 1 page.
		limits+=("$((size * 1024))")
	done

	# Set per-size limits.
	for ((i = 0; i < ${#sizes[@]}; i++)); do
		size="${sizes[$i]}"
		limit="${limits[$i]}"
		update_config '.linux.resources.hugepageLimits += [{ pagesize: "'"$size"'", limit: '"$limit"' }]'
	done

	set_cgroups_path
	runc run -d --console-socket "$CONSOLE_SOCKET" test_hugetlb
	[ "$status" -eq 0 ]

	lim="max"
	[ -v CGROUP_V1 ] && lim="limit_in_bytes"

	optional=("")
	# Add rsvd, if available.
	if test -f "$(get_cgroup_path hugetlb)/hugetlb.${sizes[0]}.rsvd.$lim"; then
		optional+=(".rsvd")
	fi

	# Check if the limits are as expected.
	for ((i = 0; i < ${#sizes[@]}; i++)); do
		size="${sizes[$i]}"
		limit="${limits[$i]}"
		for rsvd in "${optional[@]}"; do
			param="hugetlb.${size}${rsvd}.$lim"
			echo "checking $param"
			check_cgroup_value "$param" "$limit"
		done
	done
}

@test "runc run (cgroup v2 resources.unified only)" {
	requires root cgroups_v2

	set_cgroups_path
	update_config ' .linux.resources.unified |= {
				"memory.min":   "131072",
				"memory.low":   "524288",
				"memory.high": "5242880",
				"memory.max": "10485760",
				"pids.max": "99",
				"cpu.max": "10000 100000",
				"cpu.weight": "42"
			}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]

	runc exec test_cgroups_unified sh -c 'cd /sys/fs/cgroup && grep . *.min *.max *.low *.high'
	[ "$status" -eq 0 ]
	echo "$output"

	echo "$output" | grep -q '^memory.min:131072$'
	echo "$output" | grep -q '^memory.low:524288$'
	echo "$output" | grep -q '^memory.high:5242880$'
	echo "$output" | grep -q '^memory.max:10485760$'
	echo "$output" | grep -q '^pids.max:99$'
	echo "$output" | grep -q '^cpu.max:10000 100000$'

	check_systemd_value "MemoryMin" 131072
	check_systemd_value "MemoryLow" 524288
	check_systemd_value "MemoryHigh" 5242880
	check_systemd_value "MemoryMax" 10485760
	check_systemd_value "TasksMax" 99
	check_cpu_quota 10000 100000
	check_cpu_weight 42
}

@test "runc run (cgroup v2 resources.unified swap)" {
	requires root cgroups_v2 cgroups_swap

	set_cgroups_path
	update_config ' .linux.resources.unified |= {
				"memory.max": "20512768",
				"memory.swap.max": "20971520"
			}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]

	runc exec test_cgroups_unified sh -c 'cd /sys/fs/cgroup && grep . *.max'
	[ "$status" -eq 0 ]
	echo "$output"

	echo "$output" | grep -q '^memory.max:20512768$'
	echo "$output" | grep -q '^memory.swap.max:20971520$'

	check_systemd_value "MemoryMax" 20512768
	check_systemd_value "MemorySwapMax" 20971520
}

@test "runc run (cgroup v2 resources.unified override)" {
	requires root cgroups_v2

	set_cgroups_path
	# CPU shares of 3333 corresponds to CPU weight of 128.
	update_config '   .linux.resources.memory |= {"limit": 33554432}
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
			}'

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

	check_cpu_quota 5000 50000

	check_cpu_weight 42
}

@test "runc run (cgroupv2 mount inside container)" {
	requires cgroups_v2
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroups_unified
	[ "$status" -eq 0 ]

	# Make sure we don't have any extra cgroups inside
	runc exec test_cgroups_unified find /sys/fs/cgroup/ -type d
	[ "$status" -eq 0 ]
	[ "$(wc -l <<<"$output")" -eq 1 ]
}

@test "runc exec (cgroup v1+hybrid joins correct cgroup)" {
	requires root cgroups_hybrid

	set_cgroups_path

	runc run --pid-file pid.txt -d --console-socket "$CONSOLE_SOCKET" test_cgroups_group
	[ "$status" -eq 0 ]

	pid=$(cat pid.txt)
	run_cgroup=$(tail -1 </proc/"$pid"/cgroup)
	[[ "$run_cgroup" == *"runc-cgroups-integration-test"* ]]

	runc exec test_cgroups_group cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	exec_cgroup=${lines[-1]}
	[[ $exec_cgroup == *"runc-cgroups-integration-test"* ]]

	# check that the cgroups v2 path is the same for both processes
	[ "$run_cgroup" = "$exec_cgroup" ]
}

@test "runc exec should refuse a paused container" {
	requires cgroups_freezer
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]
	runc pause ct1
	[ "$status" -eq 0 ]

	# Exec should not timeout or succeed.
	runc exec ct1 echo ok
	[ "$status" -eq 255 ]
	[[ "$output" == *"cannot exec in a paused container"* ]]
}

@test "runc exec --ignore-paused" {
	requires cgroups_freezer
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]
	runc pause ct1
	[ "$status" -eq 0 ]

	# Resume the container a bit later.
	(
		sleep 2
		runc resume ct1
	) &

	# Exec should not timeout or succeed.
	runc exec --ignore-paused ct1 echo ok
	[ "$status" -eq 0 ]
	[ "$output" = "ok" ]
}

@test "runc run/create should error for a non-empty cgroup" {
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]

	# Run a second container sharing the cgroup with the first one.
	runc --debug run -d --console-socket "$CONSOLE_SOCKET" ct2
	[ "$status" -ne 0 ]
	[[ "$output" == *"container's cgroup is not empty"* ]]

	# Same but using runc create.
	runc create --console-socket "$CONSOLE_SOCKET" ct3
	[ "$status" -ne 0 ]
	[[ "$output" == *"container's cgroup is not empty"* ]]
}

@test "runc run/create should refuse pre-existing frozen cgroup" {
	requires cgroups_freezer
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	if [ -v CGROUP_V1 ]; then
		FREEZER_DIR="${CGROUP_FREEZER_BASE_PATH}/${REL_CGROUPS_PATH}"
		FREEZER="${FREEZER_DIR}/freezer.state"
		STATE="FROZEN"
	else
		FREEZER_DIR="${CGROUP_V2_PATH}"
		FREEZER="${FREEZER_DIR}/cgroup.freeze"
		STATE="1"
	fi

	# Create and freeze the cgroup.
	mkdir -p "$FREEZER_DIR"
	echo "$STATE" >"$FREEZER"

	# Start a container.
	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 1 ]
	# A warning should be printed.
	[[ "$output" == *"container's cgroup unexpectedly frozen"* ]]

	# Same check for runc create.
	runc create --console-socket "$CONSOLE_SOCKET" ct2
	[ "$status" -eq 1 ]
	# A warning should be printed.
	[[ "$output" == *"container's cgroup unexpectedly frozen"* ]]

	# Cleanup.
	rmdir "$FREEZER_DIR"
}
