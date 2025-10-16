#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# See also: "kill KILL [host pidns + init gone]" test in kill.bats.
#
# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function test_runc_delete_host_pidns() {
	requires cgroups_freezer

	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	set_cgroups_path
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		# Apparently, for rootless test, when using systemd cgroup manager,
		# newer versions of systemd clean up the container as soon as its init
		# process is gone. This is all fine and dandy, except it prevents us to
		# test this case, thus we skip the test.
		#
		# It is not entirely clear which systemd version got this feature:
		# v245 works fine, and v249 does not.
		if [ -v RUNC_USE_SYSTEMD ] && [ "$(systemd_version)" -gt 245 ]; then
			skip "rootless+systemd conflicts with systemd > 245"
		fi
		# Can't mount real /proc when rootless + no pidns,
		# so change it to a bind-mounted one from the host.
		update_config '	  .mounts |= map((select(.type == "proc")
					| .type = "none"
					| .source = "/proc"
					| .options = ["rbind", "nosuid", "nodev", "noexec"]
				  ) // .)'
	fi

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	cgpath=$(get_cgroup_path "pids")
	init_pid=$(cat "$cgpath"/cgroup.procs)

	# Start a few more processes.
	for _ in 1 2 3 4 5; do
		__runc exec -d test_busybox sleep 1h
	done

	# Now kill the container's init process. Since the container do
	# not have own PID ns, its init is no special and the container
	# will still be up and running.
	kill -9 "$init_pid"
	wait_pids_gone 10 0.2 "$init_pid"

	# Get the list of all container processes.
	mapfile -t pids < <(cat "$cgpath"/cgroup.procs)
	echo "pids:" "${pids[@]}"
	# Sanity check -- make sure all processes exist.
	for p in "${pids[@]}"; do
		kill -0 "$p"
	done

	# Must kill those processes and remove container.
	runc delete "$@" test_busybox
	[ "$status" -eq 0 ]

	runc state test_busybox
	[ "$status" -ne 0 ] # "Container does not exist"

	# Wait and check that all the processes are gone.
	wait_pids_gone 10 0.2 "${pids[@]}"

	# Make sure cgroup.procs is empty.
	mapfile -t pids < <(cat "$cgpath"/cgroup.procs || true)
	if [ ${#pids[@]} -gt 0 ]; then
		echo "expected empty cgroup.procs, got:" "${pids[@]}" 1>&2
		return 1
	fi
}

@test "runc delete" {
	# Need a permission to create a cgroup.
	# XXX(@kolyshkin): currently this test does not handle rootless when
	# fs cgroup driver is used, because in this case cgroup (with a
	# predefined name) is created by tests/rootless.sh, not by runc.
	[ $EUID -ne 0 ] && requires systemd
	set_resources_limit

	runc run -d --console-socket "$CONSOLE_SOCKET" testbusyboxdelete
	[ "$status" -eq 0 ]

	testcontainer testbusyboxdelete running
	# Ensure the find statement used later is correct.
	output=$(find /sys/fs/cgroup -name testbusyboxdelete -o -name \*-testbusyboxdelete.scope 2>/dev/null || true)
	if [ -z "$output" ]; then
		fail "expected cgroup not found"
	fi

	runc kill testbusyboxdelete KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 testbusyboxdelete stopped

	runc delete testbusyboxdelete
	[ "$status" -eq 0 ]

	runc state testbusyboxdelete
	[ "$status" -ne 0 ]

	output=$(find /sys/fs/cgroup -name testbusyboxdelete -o -name \*-testbusyboxdelete.scope 2>/dev/null || true)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# check state
	testcontainer test_busybox running

	# force delete test_busybox
	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]
}

@test "runc delete --force ignore not exist" {
	runc delete --force notexists
	[ "$status" -eq 0 ]
}

# Issue 4047, case "runc delete".
@test "runc delete [host pidns + init gone]" {
	test_runc_delete_host_pidns
}

# Issue 4047, case "runc delete --force" (different code path).
@test "runc delete --force [host pidns + init gone]" {
	test_runc_delete_host_pidns --force
}

@test "runc delete --force [paused container]" {
	runc run -d --console-socket "$CONSOLE_SOCKET" ct1
	[ "$status" -eq 0 ]
	testcontainer ct1 running

	runc pause ct1
	runc delete --force ct1
	[ "$status" -eq 0 ]
}

@test "runc delete --force in cgroupv1 with subcgroups" {
	requires cgroups_v1 root cgroupns
	set_cgroups_path
	set_cgroup_mount_writable
	# enable cgroupns
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	local subsystems="memory freezer"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	__runc exec -d test_busybox sleep 1d

	# find the pid of sleep
	pid=$(__runc exec test_busybox ps -a | grep 1d | awk '{print $1}')
	[[ ${pid} =~ [0-9]+ ]]

	# create a sub-cgroup
	cat <<EOF | runc exec test_busybox sh
set -e -u -x
for s in ${subsystems}; do
  cd /sys/fs/cgroup/\$s
  mkdir foo
  cd foo
  echo ${pid} > tasks
  cat tasks
done
EOF
	[ "$status" -eq 0 ]
	[[ "$output" =~ [0-9]+ ]]

	for s in ${subsystems}; do
		name=CGROUP_${s^^}_BASE_PATH
		eval path=\$"${name}${REL_CGROUPS_PATH}/foo"
		# shellcheck disable=SC2154
		[ -d "${path}" ] || fail "test failed to create memory sub-cgroup ($path not found)"
	done

	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]

	output=$(find /sys/fs/cgroup -wholename '*testbusyboxdelete*' -type d 2>/dev/null || true)
	[ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force in cgroupv2 with subcgroups" {
	requires cgroups_v2 root
	set_cgroups_path
	set_cgroup_mount_writable

	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# check state
	testcontainer test_busybox running

	# create a sub process
	__runc exec -d test_busybox sleep 1d

	# find the pid of sleep
	pid=$(__runc exec test_busybox ps -a | grep 1d | awk '{print $1}')
	[[ ${pid} =~ [0-9]+ ]]

	# create subcgroups
	cat <<EOF >nest.sh
  set -e -u -x
  cd /sys/fs/cgroup
  echo +pids > cgroup.subtree_control
  mkdir foo
  cd foo
  echo threaded > cgroup.type
  echo ${pid} > cgroup.threads
  cat cgroup.threads
EOF
	runc exec test_busybox sh <nest.sh
	[ "$status" -eq 0 ]
	[[ "$output" =~ [0-9]+ ]]

	# check create subcgroups success
	[ -d "$CGROUP_V2_PATH"/foo ]

	# force delete test_busybox
	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]

	# check delete subcgroups success
	[ ! -d "$CGROUP_V2_PATH"/foo ]
}

@test "runc delete removes failed systemd unit" {
	requires systemd_v244 # Older systemd lacks RuntimeMaxSec support.

	set_cgroups_path
	update_config '	  .annotations += {
				"org.systemd.property.RuntimeMaxSec": "2",
				"org.systemd.property.TimeoutStopSec": "1"
			   }
			| .process.args |= ["/bin/sleep", "10"]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test-failed-unit
	[ "$status" -eq 0 ]

	wait_for_container 10 1 test-failed-unit stopped

	local user=""
	[ $EUID -ne 0 ] && user="--user"

	# Expect "unit is not active" exit code.
	run -3 systemctl status $user "$SD_UNIT_NAME"

	runc delete test-failed-unit
	# Expect "no such unit" exit code.
	run -4 systemctl status $user "$SD_UNIT_NAME"
}


## runc delete multiple stopped containers
@test "runc delete multiple stopped containers" {
	[ $EUID -ne 0 ] && requires systemd
	set_resources_limit
	
	local containers=("multi-stopped-1" "multi-stopped-2" "multi-stopped-3")

	for name in "${containers[@]}"; do
		runc run -d --console-socket "$CONSOLE_SOCKET" "$name"
		[ "$status" -eq 0 ]
		testcontainer "$name" running
		runc kill "$name" KILL
		wait_for_container 10 1 "$name" stopped
	done

	runc delete "${containers[@]}"
	[ "$status" -eq 0 ]

	for name in "${containers[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name should be deleted"
	done
}

## runc delete --force multiple running/paused containers
@test "runc delete --force multiple running and paused" {
	local containers=("multi-force-1" "multi-force-2" "multi-force-3")

	runc run -d --console-socket "$CONSOLE_SOCKET" "${containers[0]}" 
	[ "$status" -eq 0 ]
	testcontainer "${containers[0]}" running

	runc run -d --console-socket "$CONSOLE_SOCKET" "${containers[1]}" 
	[ "$status" -eq 0 ]
	runc pause "${containers[1]}"
	testcontainer "${containers[1]}" paused

	runc run -d --console-socket "$CONSOLE_SOCKET" "${containers[2]}" 
	[ "$status" -eq 0 ]
	testcontainer "${containers[2]}" running

	runc delete --force "${containers[@]}"
	[ "$status" -eq 0 ]

	for name in "${containers[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name should be force-deleted"
	done
}

function run_multiple_host_pidns_test() {
	local force_flag="$1"
	local containers=("multi-hostpid-1" "multi-hostpid-2")
	local all_pids=()

	for name in "${containers[@]}"; do
		requires cgroups_freezer
		
		update_config '.linux.namespaces -= [{"type": "pid"}]'
		set_cgroups_path
		if [ $EUID -ne 0 ]; then
			requires rootless_cgroup
			if [ -v RUNC_USE_SYSTEMD ] && [ "$(systemd_version)" -gt 245 ]; then
				skip "rootless+systemd conflicts with systemd > 245"
			fi
			update_config '.mounts |= map((select(.type == "proc")
						| .type = "none"
						| .source = "/proc"
						| .options = ["rbind", "nosuid", "nodev", "noexec"]
					 ) // .)'
		fi
		
		runc run -d --console-socket "$CONSOLE_SOCKET" "$name"
		[ "$status" -eq 0 ]
		cgpath=$(get_cgroup_path "pids" "$name")
		init_pid=$(cat "$cgpath"/cgroup.procs)

		__runc exec -d "$name" sleep 1h
		__runc exec -d "$name" sleep 1h

		kill -9 "$init_pid"
		wait_pids_gone 10 0.2 "$init_pid"

		mapfile -t pids_for_ct < <(cat "$cgpath"/cgroup.procs)
		all_pids+=("${pids_for_ct[@]}")
	done
	
	runc delete $force_flag "${containers[@]}"
	[ "$status" -eq 0 ]

	for name in "${containers[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name should be deleted"
	done
	
	wait_pids_gone 10 0.2 "${all_pids[@]}"
}

@test "runc delete multiple containers [host pidns + init gone]" {
	run_multiple_host_pidns_test ""
}

@test "runc delete --force multiple containers [host pidns + init gone]" {
	run_multiple_host_pidns_test "--force"
}


function run_multiple_cgroupv1_test() {
	local force_flag="$1"
	requires cgroups_v1 root cgroupns
	set_cgroups_path
	set_cgroup_mount_writable
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	local containers=("multi-cg1-1" "multi-cg1-2" "multi-cg1-3")
	local subsystems="memory freezer"

	for name in "${containers[@]}"; do
		runc run -d --console-socket "$CONSOLE_SOCKET" "$name"
		[ "$status" -eq 0 ]
		testcontainer "$name" running

		__runc exec -d "$name" sleep 1d

		pid=$(__runc exec "$name" ps -a | grep 1d | awk '{print $1}')
		[[ ${pid} =~ [0-9]+ ]]

		cat <<EOF | runc exec "$name" sh
set -e -u -x
for s in ${subsystems}; do
  cd /sys/fs/cgroup/\$s
  mkdir foo
  cd foo
  echo ${pid} > tasks
done
EOF
		[ "$status" -eq 0 ]
	done

	runc delete $force_flag "${containers[@]}"
	[ "$status" -eq 0 ]

	for name in "${containers[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name should be deleted"
	done

	for s in ${subsystems}; do
		for name in "${containers[@]}"; do
			path=$(eval echo "\$CGROUP_${s^^}_BASE_PATH/${name}/foo")
			[ ! -d "$path" ] || fail "Sub-cgroup $path not cleaned"
		done
	done
}

@test "runc delete --force multiple containers in cgroupv1 with subcgroups" {
	run_multiple_cgroupv1_test "--force"
}

@test "runc delete multiple containers in cgroupv1 with subcgroups" {
	run_multiple_cgroupv1_test ""
}


function run_multiple_cgroupv2_test() {
	local force_flag="$1"
	requires cgroups_v2 root
	set_cgroup_mount_writable

	local containers=("multi-cg2-1" "multi-cg2-2")

	for name in "${containers[@]}"; do
		echo "Starting container $name" 1>&2
		
		set_cgroups_path
		
		runc run -d --console-socket "$CONSOLE_SOCKET" "$name"
		[ "$status" -eq 0 ] || fail "Failed to start container $name"
		testcontainer "$name" running

		local container_cg_path
		container_cg_path=$(get_cgroup_path "" "$name")

		__runc exec -d "$name" sleep 1d

		pid=$(__runc exec "$name" ps -a | grep 1d | awk '{print $1}')
		[[ ${pid} =~ [0-9]+ ]] || fail "Failed to get pid of process in container $name"

		local sub_cgroup_name="sub-$name"
		echo "Creating sub-cgroups for $name as $sub_cgroup_name" 1>&2
		
		cat <<EOF > nest_${name}.sh
set -e -u -x
cd /sys/fs/cgroup
echo +pids > cgroup.subtree_control
mkdir $sub_cgroup_name
cd $sub_cgroup_name
echo threaded > cgroup.type
echo ${pid} > cgroup.threads
EOF
		runc exec "$name" sh < nest_${name}.sh
		[ "$status" -eq 0 ] || fail "Failed to create sub-cgroup for $name"
		
		[ -d "$container_cg_path/$sub_cgroup_name" ] || fail "cgroupv2 $sub_cgroup_name not created for $name at $container_cg_path/$sub_cgroup_name"
	done

	echo "Deleting containers ${containers[@]}" 1>&2
	local force_args=""
	[ "$force_flag" = "--force" ] && force_args="--force"
	
	runc delete $force_args "${containers[@]}"
	[ "$status" -eq 0 ] || fail "Failed to delete containers ${containers[@]}"

	echo "Verifying cleanup for cgroups" 1>&2
	for name in "${containers[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name should be deleted"

		local container_cg_path_after_delete
		container_cg_path_after_delete=$(get_cgroup_path "" "$name" || true)
		
		[ ! -d "$container_cg_path_after_delete" ] || fail "Main Cgroup directory $container_cg_path_after_delete not cleaned up"
	done

	output=$(find /sys/fs/cgroup -wholename "*multi-cg2*" -type d 2>/dev/null || true)
	[ -z "$output" ] || fail "Cgroup directories not cleaned up correctly: $output"

	rm -f nest_multi-cg2-*.sh
}

@test "runc delete --force multiple containers in cgroupv2 with subcgroups" {
	run_multiple_cgroupv2_test "--force"
}
