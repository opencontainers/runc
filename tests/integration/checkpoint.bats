#!/usr/bin/env bats

load helpers

function create_netns() {
	# Create a temporary name for the test network namespace.
	tmp=$(mktemp -u)
	ns_name=$(basename "$tmp")

	# Create the network namespace.
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')
}

function delete_netns() {
	# Delete the namespace only if the ns_name variable is set.
	[ -v ns_name ] && ip netns del "$ns_name"
}

function setup() {
	# XXX: currently criu require root containers.
	requires criu root

	setup_busybox

	# Create a dummy interface to move to the container.
	ip link add dummy0 type dummy
}

function teardown() {
	ip link del dev dummy0
	delete_netns
	teardown_bundle
	rm -f /etc/criu/default.conf
}

function setup_pipes() {
	# The changes to 'terminal' are needed for running in detached mode
	# shellcheck disable=SC2016
	update_config ' (.. | select(.terminal? != null)) .terminal |= false
			| (.. | select(.[]? == "sh")) += ["-c", "for i in `seq 10`; do read xxx || continue; echo ponG $xxx; done"]'

	# Create three sets of pipes for __runc run.
	# for stderr
	exec {pipe}<> <(:)
	exec {err_r}</proc/self/fd/$pipe
	exec {err_w}>/proc/self/fd/$pipe
	exec {pipe}>&-
	# for stdout
	exec {pipe}<> <(:)
	exec {out_r}</proc/self/fd/$pipe
	exec {out_w}>/proc/self/fd/$pipe
	exec {pipe}>&-
	# for stdin
	exec {pipe}<> <(:)
	exec {in_r}</proc/self/fd/$pipe
	exec {in_w}>/proc/self/fd/$pipe
	exec {pipe}>&-
}

function check_pipes() {
	local output stderr

	echo Ping >&${in_w}
	exec {in_w}>&-
	exec {out_w}>&-
	exec {err_w}>&-

	exec {in_r}>&-
	output=$(cat <&${out_r})
	exec {out_r}>&-
	stderr=$(cat <&${err_r})
	exec {err_r}>&-

	[[ "${output}" == *"ponG Ping"* ]]
	if [ -n "$stderr" ]; then
		fail "runc stderr: $stderr"
	fi
}

# Usage: runc_run_with_pipes container-name
function runc_run_with_pipes() {
	# Start a container to be checkpointed, with stdin/stdout redirected
	# so that check_pipes can be used to check it's working fine.
	# We have to redirect stderr as well because otherwise it is
	# redirected to a bats log file, which is not accessible to CRIU
	# (i.e. outside of container) so checkpointing will fail.
	ret=0
	__runc run -d "$1" <&${in_r} >&${out_w} 2>&${err_w} || ret=$?
	if [ "$ret" -ne 0 ]; then
		echo "runc run -d $1 (status: $ret):"
		exec {err_w}>&-
		cat <&${err_r}
		fail "runc run failed"
	fi

	testcontainer "$1" running
}

# Usage: runc_restore_with_pipes work-dir container-name [optional-arguments ...]
function runc_restore_with_pipes() {
	workdir="$1"
	shift
	name="$1"
	shift

	ret=0
	__runc restore -d --work-path "$workdir" --image-path ./image-dir "$@" "$name" <&${in_r} >&${out_w} 2>&${err_w} || ret=$?
	if [ "$ret" -ne 0 ]; then
		echo "__runc restore $name failed (status: $ret)"
		exec {err_w}>&-
		cat <&${err_r}
		fail "runc restore failed"
	fi

	testcontainer "$name" running

	runc -0 exec --cwd /bin "$name" echo ok
	[ "$output" = "ok" ]
}

function simple_cr() {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	for _ in $(seq 2); do
		runc -0 "$@" checkpoint --work-path ./work-dir test_busybox

		testcontainer test_busybox checkpointed

		runc -0 "$@" restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox

		testcontainer test_busybox running
	done
}

function simple_cr_with_netdevice() {
	# Set custom parameters to the netdevice to validate those are respected
	mtu_value=1789
	mac_address="00:11:22:33:44:55"
	global_ip="169.254.169.77/32"

	ip link set mtu "$mtu_value" dev dummy0
	ip link set address "$mac_address" dev dummy0
	ip address add "$global_ip" dev dummy0

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'
	update_config ' .linux.netDevices |= {"dummy0": {} }'
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox_netdevice

	testcontainer test_busybox_netdevice running
	runc -0 exec test_busybox_netdevice ip address show dev dummy0
	[[ "$output" == *" $global_ip "* ]]
	[[ "$output" == *"ether $mac_address "* ]]
	[[ "$output" == *"mtu $mtu_value "* ]]

	for _ in $(seq 2); do
		runc -0 "$@" checkpoint --work-path ./work-dir test_busybox_netdevice

		testcontainer test_busybox_netdevice checkpointed

		runc -0 "$@" restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox_netdevice

		testcontainer test_busybox_netdevice running
		runc -0 exec test_busybox_netdevice ip address show dev dummy0
		[[ "$output" == *" $global_ip "* ]]
		[[ "$output" == *"ether $mac_address "* ]]
		[[ "$output" == *"mtu $mtu_value "* ]]
	done
}

@test "checkpoint and restore" {
	ls -l /etc/criu/ || true
	tail /etc/criu/* || true
	ls -l ~/.criu/ || true
	tail ~/.criu/* || true

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	for _ in $(seq 2); do
		runc -0 "$@" checkpoint --work-path ./work-dir test_busybox
		testcontainer test_busybox checkpointed

		runc -0 "$@" restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox
		testcontainer test_busybox running
	done
}

@test "checkpoint and restore (bind mount, destination is symlink)" {
	mkdir -p rootfs/real/conf
	ln -s /real/conf rootfs/conf
	update_config '	  .mounts += [{
					source: ".",
					destination: "/conf",
					options: ["bind"]
				}]'
	simple_cr
}

@test "checkpoint and restore (with --debug)" {
	simple_cr --debug
}

@test "checkpoint and restore (cgroupns)" {
	# cgroupv2 already enables cgroupns so this case was tested above already
	requires cgroups_v1 cgroupns

	# enable CGROUPNS
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	simple_cr
}

@test "checkpoint and restore with netdevice" {
	simple_cr_with_netdevice
}

@test "checkpoint and restore with netdevice (bind mount, destination is symlink)" {
	mkdir -p rootfs/real/conf
	ln -s /real/conf rootfs/conf
	update_config '	  .mounts += [{
					source: ".",
					destination: "/conf",
					options: ["bind"]
				}]'
	simple_cr_with_netdevice
}

@test "checkpoint and restore with netdevice (with --debug)" {
	simple_cr_with_netdevice --debug
}

@test "checkpoint and restore with netdevice (cgroupns)" {
	# cgroupv2 already enables cgroupns so this case was tested above already
	requires cgroups_v1 cgroupns

	# enable CGROUPNS
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	simple_cr_with_netdevice
}

@test "checkpoint --pre-dump (bad --parent-path)" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	# runc should fail with absolute parent image path.
	runc ! checkpoint --parent-path "$(pwd)"/parent-dir --work-path ./work-dir --image-path ./image-dir test_busybox
	[[ "${output}" == *"--parent-path"* ]]

	# runc should fail with invalid parent image path.
	runc ! checkpoint --parent-path ./parent-dir --work-path ./work-dir --image-path ./image-dir test_busybox
	[[ "${output}" == *"--parent-path"* ]]
}

@test "checkpoint --pre-dump and restore" {
	# Requires kernel dirty memory tracking (missing on ARM, see
	# https://github.com/checkpoint-restore/criu/issues/1729).
	requires criu_feature_mem_dirty_track

	setup_pipes
	runc_run_with_pipes test_busybox

	mkdir parent-dir
	runc -0 checkpoint --pre-dump --image-path ./parent-dir test_busybox

	testcontainer test_busybox running

	mkdir image-dir
	mkdir work-dir
	runc -0 checkpoint --parent-path ../parent-dir --work-path ./work-dir --image-path ./image-dir test_busybox

	# check parent path is valid
	[ -e ./image-dir/parent ]

	testcontainer test_busybox checkpointed

	runc_restore_with_pipes ./work-dir test_busybox
	check_pipes
}

@test "checkpoint --lazy-pages and restore" {
	# Requires lazy-pages support.
	requires criu_feature_uffd-noncoop

	setup_pipes
	runc_run_with_pipes test_busybox

	mkdir image-dir
	mkdir work-dir

	# For lazy migration we need to know when CRIU is ready to serve
	# the memory pages via TCP.
	exec {pipe}<> <(:)
	# shellcheck disable=SC2094
	exec {lazy_r}</proc/self/fd/$pipe {lazy_w}>/proc/self/fd/$pipe
	exec {pipe}>&-

	# TCP port for lazy migration
	port=27277

	__runc checkpoint \
		--lazy-pages \
		--page-server 0.0.0.0:${port} \
		--status-fd ${lazy_w} \
		--manage-cgroups-mode=ignore \
		--work-path ./work-dir \
		--image-path ./image-dir \
		test_busybox &
	cpt_pid=$!

	# wait for lazy page server to be ready
	out=$(timeout 2 dd if=/proc/self/fd/${lazy_r} bs=1 count=1 2>/dev/null | od)
	exec {lazy_r}>&-
	exec {lazy_w}>&-
	# shellcheck disable=SC2116,SC2086
	out=$(echo $out) # rm newlines
	# expecting \0 which od prints as
	[ "$out" = "0000000 000000 0000001" ]

	# Check if inventory.img was written
	[ -e image-dir/inventory.img ]

	# Start CRIU in lazy-daemon mode
	criu lazy-pages --page-server --address 127.0.0.1 --port ${port} -D image-dir &
	lp_pid=$!

	# Restore lazily from checkpoint.
	#
	# The restored container needs a different name and a different cgroup
	# (and a different systemd unit name, in case systemd cgroup driver is
	# used) as the checkpointed container is not yet destroyed. It is only
	# destroyed at that point in time when the last page is lazily
	# transferred to the destination.
	#
	# Killing the CRIU on the checkpoint side will let the container
	# continue to run if the migration failed at some point.
	runc_restore_with_pipes ./image-dir test_busybox_restore \
		--lazy-pages \
		--manage-cgroups-mode=ignore

	wait $cpt_pid

	wait $lp_pid

	check_pipes
}

@test "checkpoint and restore in external network namespace" {
	# Requires external network namespaces (criu >= 3.10).
	requires criu_feature_external_net_ns

	# create a temporary name for the test network namespace
	tmp=$(mktemp)
	rm -f "$tmp"
	ns_name=$(basename "$tmp")
	# create network namespace
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')
	# shellcheck disable=SC2012
	ns_inode=$(ls -iL "$ns_path" | awk '{ print $1 }')

	# tell runc which network namespace to use
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	for _ in $(seq 2); do
		# checkpoint the running container; this automatically tells CRIU to
		# handle the network namespace defined in config.json as an external
		runc -0 checkpoint --work-path ./work-dir test_busybox

		testcontainer test_busybox checkpointed

		# restore from checkpoint; this should restore the container into the existing network namespace
		runc -0 restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox

		testcontainer test_busybox running

		# container should be running in same network namespace as before
		pid=$(__runc state test_busybox | jq '.pid')
		ns_inode_new=$(readlink /proc/"$pid"/ns/net | sed -e 's/.*\[\(.*\)\]/\1/')
		echo "old network namespace inode $ns_inode"
		echo "new network namespace inode $ns_inode_new"
		[ "$ns_inode" -eq "$ns_inode_new" ]
	done
	ip netns del "$ns_name"
}

@test "checkpoint and restore with container specific CRIU config" {
	tmp=$(mktemp /tmp/runc-criu-XXXXXX.conf)
	# Name we write to /etc/criu/default.conf.
	tmplog1=$(mktemp /tmp/runc-criu-log-XXXXXX.log)
	unlink "$tmplog1"
	tmplog1=$(basename "$tmplog1")
	# Actual configuration file name to be used.
	tmplog2=$(mktemp /tmp/runc-criu-log-XXXXXX.log)
	unlink "$tmplog2"
	tmplog2=$(basename "$tmplog2")
	# Add the annotation to set a container specific CRIU config file.
	update_config '.annotations += {"org.criu.config": "'"$tmp"'"}'

	# Tell CRIU to use another configuration file.
	mkdir -p /etc/criu
	echo "log-file=$tmplog1" >/etc/criu/default.conf
	# Make sure the RPC defined configuration file overwrites the default.conf.
	echo "log-file=$tmplog2" >"$tmp"

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 checkpoint --work-path ./work-dir test_busybox
	run ! test -f ./work-dir/"$tmplog1"
	test -f ./work-dir/"$tmplog2"

	testcontainer test_busybox checkpointed

	test -f ./work-dir/"$tmplog2" && unlink ./work-dir/"$tmplog2"

	runc -0 restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox
	run ! test -f ./work-dir/"$tmplog1"
	test -f ./work-dir/"$tmplog2"

	testcontainer test_busybox running
	unlink "$tmp"
	test -f ./work-dir/"$tmplog2" && unlink ./work-dir/"$tmplog2"
}

@test "checkpoint and restore with nested bind mounts" {
	bind1=$(mktemp -d -p .)
	bind2=$(mktemp -d -p .)
	update_config '	  .mounts += [{
					type: "bind",
					source: "'"$bind1"'",
					destination: "/test",
					options: ["rw", "bind"]
				},
	                        {
					type: "bind",
					source: "'"$bind2"'",
					destination: "/test/for/nested",
					options: ["rw", "bind"]
				}]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 checkpoint --work-path ./work-dir test_busybox

	testcontainer test_busybox checkpointed

	# cleanup mountpoints created by runc during creation
	# the mountpoints should be recreated during restore - that is the actual thing tested here
	rm -rf "${bind1:?}"/*

	runc -0 restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running
}

@test "checkpoint then restore into a different cgroup (via --manage-cgroups-mode ignore)" {
	set_resources_limit
	set_cgroups_path
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	local orig_path
	orig_path=$(get_cgroup_path "pids")
	# Check that the cgroup exists.
	test -d "$orig_path"

	runc -0 checkpoint --work-path ./work-dir --manage-cgroups-mode ignore test_busybox
	testcontainer test_busybox checkpointed
	# Check that the cgroup is gone.
	run ! test -d "$orig_path"

	# Restore into a different cgroup.
	set_cgroups_path # Changes the path.
	runc -0 restore -d --manage-cgroups-mode ignore --pid-file pid \
		--work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	# Check that the old cgroup path doesn't exist.
	run ! test -d "$orig_path"

	# Check that the new path exists.
	local new_path
	new_path=$(get_cgroup_path "pids")
	test -d "$new_path"

	# Check that container's init is in the new cgroup.
	local pid
	pid=$(cat "pid")
	grep -q "${REL_CGROUPS_PATH}$" "/proc/$pid/cgroup"
}

@test "checkpoint/restore and exec" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	local execed_pid=""
	for _ in $(seq 2); do
		runc -0 checkpoint --work-path ./work-dir test_busybox

		testcontainer test_busybox checkpointed

		runc -0 restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox

		testcontainer test_busybox running

		# verify that previously exec'd process is restored.
		if [ -n "$execed_pid" ]; then
			runc -0 exec test_busybox ls -ld "/proc/$execed_pid"
		fi

		# exec a new background process.
		runc -0 exec test_busybox sh -c 'sleep 1000 < /dev/null &> /dev/null & echo $!'
		execed_pid=$output
	done
}
