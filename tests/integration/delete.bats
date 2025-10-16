#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# ------------------------------
# Helper: Prepare host PIDNS container
# ------------------------------
# See also: "kill KILL [host pidns + init gone]" test in kill.bats.
#
# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function prepare_host_pidns_container() {
	local name="$1"
	requires cgroups_freezer

	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	set_cgroups_path
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		if [ -v RUNC_USE_SYSTEMD ] && [ "$(systemd_version)" -gt 245 ]; then
			skip "rootless+systemd conflicts with systemd > 245"
		fi
		update_config '	  .mounts |= map((select(.type == "proc")
					| .type = "none"
					| .source = "/proc"
					| .options = ["rbind", "nosuid", "nodev", "noexec"]
				  ) // .)'
	fi

	runc run -d --console-socket "$CONSOLE_SOCKET" "$name"
	[ "$status" -eq 0 ]

	local cgpath init_pid
	cgpath=$(get_cgroup_path "pids" "$name")
	init_pid=$(cat "$cgpath"/cgroup.procs)

	for _ in 1 2; do
		__runc exec -d "$name" sleep 1h
	done

	kill -9 "$init_pid"
	wait_pids_gone 10 0.2 "$init_pid"

	mapfile -t pids < <(cat "$cgpath"/cgroup.procs)
	for p in "${pids[@]}"; do
		kill -0 "$p"
	done

	echo "$name:${pids[*]}"
}

# ------------------------------
# Helper: Batch delete containers
# ------------------------------
function batch_delete_and_verify() {
	local force_flag="$1"; shift
	local containers=("$@")
	local force_args=""
	[ "$force_flag" = "force" ] && force_args="--force"

	runc delete $force_args "${containers[@]}"
	[ "$status" -eq 0 ] || fail "Batch delete failed"

	for name in "${containers[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name was not deleted"

		if [[ -n "${ALL_PIDS[$name]}" ]]; then
			wait_pids_gone 10 0.2 ${ALL_PIDS[$name]}
		fi

		if [ -d "$CGROUP_V1_PATH/$name" ]; then
			[ ! -d "$CGROUP_V1_PATH/$name" ] || fail "Cgroup $CGROUP_V1_PATH/$name not removed"
		fi
		if [ -d "$CGROUP_V2_PATH/$name" ]; then
			[ ! -d "$CGROUP_V2_PATH/$name" ] || fail "Cgroup $CGROUP_V2_PATH/$name not removed"
		fi

		local user=""
		[ "$EUID" -ne 0 ] && user="--user"
		run -4 systemctl status $user "runc-$name.scope"
		[ "$status" -eq 4 ] || true
	done
}

# ------------------------------
# Host PIDNS / normal container batch delete
# ------------------------------
@test "runc delete [host pidns + init gone] multiple containers" {
	local ct1 ct2 ct3
	ct1=$(prepare_host_pidns_container "ct-hostpid-1")
	ct2=$(prepare_host_pidns_container "ct-hostpid-2")
	ct3=$(prepare_host_pidns_container "ct-hostpid-3")

	declare -A ALL_PIDS
	ALL_PIDS["${ct1%%:*}"]="${ct1#*:}"
	ALL_PIDS["${ct2%%:*}"]="${ct2#*:}"
	ALL_PIDS["${ct3%%:*}"]="${ct3#*:}"

	batch_delete_and_verify "" "ct-hostpid-1" "ct-hostpid-2" "ct-hostpid-3"
}

@test "runc delete --force [host pidns + init gone] multiple containers" {
	local ct1 ct2
	ct1=$(prepare_host_pidns_container "ct-hostpid-f1")
	ct2=$(prepare_host_pidns_container "ct-hostpid-f2")

	declare -A ALL_PIDS
	ALL_PIDS["${ct1%%:*}"]="${ct1#*:}"
	ALL_PIDS["${ct2%%:*}"]="${ct2#*:}"

	batch_delete_and_verify "force" "ct-hostpid-f1" "ct-hostpid-f2"
}

# ------------------------------
# Cgroup v1 batch delete with subgroups
# ------------------------------
@test "runc delete --force in cgroupv1 with multiple containers and subcgroups" {
	requires cgroups_v1 root cgroupns
	set_cgroup_mount_writable

	local container_list=("ct-cgv1-1" "ct-cgv1-2")
	local subsystems="memory freezer"

	for name in "${container_list[@]}"; do
		set_cgroups_path
		update_config '.linux.namespaces += [{"type": "cgroup"}]'

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
  mkdir sub-foo
  cd sub-foo
  echo ${pid} > tasks
done
EOF
		[ "$status" -eq 0 ]

        for s in ${subsystems}; do
			name_upper=CGROUP_${s^^}_BASE_PATH
			eval path=\$"${name_upper}${REL_CGROUPS_PATH}/$name/sub-foo"
			[ -d "${path}" ] || fail "Test failed to create $s sub-cgroup for $name"
		done
	done

	runc delete --force "${container_list[@]}"
	[ "$status" -eq 0 ] || fail "Batch delete failed"

	for name in "${container_list[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name was not deleted"

        for s in ${subsystems}; do
			name_upper=CGROUP_${s^^}_BASE_PATH
			eval path=\$"${name_upper}${REL_CGROUPS_PATH}/$name"
			[ ! -d "${path}" ] || fail "Main Cgroup $path not removed by recursive delete."
		done
	done
}

# ------------------------------
# Cgroup v2 batch delete with subgroups
# ------------------------------
@test "runc delete --force in cgroupv2 with multiple containers and subcgroups" {
	requires cgroups_v2 root
	set_cgroup_mount_writable

	local container_list=("ct-cgv2-1" "ct-cgv2-2")

	for name in "${container_list[@]}"; do
		set_cgroups_path

		runc run -d --console-socket "$CONSOLE_SOCKET" "$name"
		[ "$status" -eq 0 ]
		testcontainer "$name" running

		__runc exec -d "$name" sleep 1d
		pid=$(__runc exec "$name" ps -a | grep 1d | awk '{print $1}')
		[[ ${pid} =~ [0-9]+ ]]

		cat <<EOF >nest-$name.sh
set -e -u -x
cd /sys/fs/cgroup
echo +pids > cgroup.subtree_control
mkdir sub-foo
cd sub-foo
echo threaded > cgroup.type
echo ${pid} > cgroup.threads
EOF
		runc exec "$name" sh <nest-$name.sh
		[ "$status" -eq 0 ]
        
        [ -d "$CGROUP_V2_PATH/$name/sub-foo" ] || fail "Subcgroup creation failed for $name"
	done

	runc delete --force "${container_list[@]}"
	[ "$status" -eq 0 ] || fail "Batch delete failed"

	for name in "${container_list[@]}"; do
		runc state "$name"
		[ "$status" -ne 0 ] || fail "Container $name was not deleted"
        
		[ ! -d "$CGROUP_V2_PATH/$name" ] || fail "Cgroup $CGROUP_V2_PATH/$name not removed by recursive delete."
	done

	rm -f nest-*.sh
}
