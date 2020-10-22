#!/usr/bin/env bats

load helpers

function setup() {
	# XXX: currently criu require root containers.
	requires criu root

	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
	teardown_running_container test_busybox_restore
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
	echo Ping >&${in_w}
	exec {in_w}>&-
	exec {out_w}>&-
	output=$(cat <&${out_r})
	[[ "${output}" == *"ponG Ping"* ]]
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
	__runc --criu "$CRIU" restore -d --work-path "$workdir" --image-path ./image-dir "$@" "$name" <&${in_r} >&${out_w} 2>&${err_w} || ret=$?
	if [ "$ret" -ne 0 ]; then
		echo "__runc restore $name failed (status: $ret)"
		exec {err_w}>&-
		cat <&${err_r}
		echo "CRIU restore log errors (if any):"
		grep -B 5 Error "$workdir"/restore.log || true
		fail "runc restore failed"
	fi

	testcontainer "$name" running

	runc exec --cwd /bin "$name" echo ok
	[ "$status" -eq 0 ]
	[[ ${output} == "ok" ]]
}

function simple_cr() {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	for _ in $(seq 2); do
		# checkpoint the running container
		runc --criu "$CRIU" checkpoint --work-path ./work-dir test_busybox
		grep -B 5 Error ./work-dir/dump.log || true
		[ "$status" -eq 0 ]

		# after checkpoint busybox is no longer running
		testcontainer test_busybox checkpointed

		# restore from checkpoint
		runc --criu "$CRIU" restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox
		grep -B 5 Error ./work-dir/restore.log || true
		[ "$status" -eq 0 ]

		# busybox should be back up and running
		testcontainer test_busybox running
	done
}

@test "checkpoint and restore " {
	simple_cr
}

@test "checkpoint and restore (cgroupns)" {
	# cgroupv2 already enables cgroupns so this case was tested above already
	requires cgroups_v1 cgroupns

	# enable CGROUPNS
	update_config '.linux.namespaces += [{"type": "cgroup"}]'

	simple_cr
}

@test "checkpoint --pre-dump and restore" {
	setup_pipes
	runc_run_with_pipes test_busybox

	#test checkpoint pre-dump
	mkdir parent-dir
	runc --criu "$CRIU" checkpoint --pre-dump --image-path ./parent-dir test_busybox
	[ "$status" -eq 0 ]

	# busybox should still be running
	testcontainer test_busybox running

	# checkpoint the running container
	mkdir image-dir
	mkdir work-dir
	runc --criu "$CRIU" checkpoint --parent-path ./parent-dir --work-path ./work-dir --image-path ./image-dir test_busybox
	grep -B 5 Error ./work-dir/dump.log || true
	[ "$status" -eq 0 ]

	# after checkpoint busybox is no longer running
	testcontainer test_busybox checkpointed

	runc_restore_with_pipes ./work-dir test_busybox
	check_pipes
}

@test "checkpoint --lazy-pages and restore" {
	# check if lazy-pages is supported
	if ! "${CRIU}" check --feature uffd-noncoop; then
		skip "this criu does not support lazy migration"
	fi

	setup_pipes
	runc_run_with_pipes test_busybox

	# checkpoint the running container
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

	__runc --criu "$CRIU" checkpoint --lazy-pages --page-server 0.0.0.0:${port} --status-fd ${lazy_w} --work-path ./work-dir --image-path ./image-dir test_busybox &
	cpt_pid=$!

	# wait for lazy page server to be ready
	out=$(timeout 2 dd if=/proc/self/fd/${lazy_r} bs=1 count=1 2>/dev/null | od)
	exec {lazy_w}>&-
	# shellcheck disable=SC2116,SC2086
	out=$(echo $out) # rm newlines
	# show errors if there are any before we fail
	grep -B5 Error ./work-dir/dump.log || true
	# expecting \0 which od prints as
	[ "$out" = "0000000 000000 0000001" ]

	# Check if inventory.img was written
	[ -e image-dir/inventory.img ]

	# Start CRIU in lazy-daemon mode
	${CRIU} lazy-pages --page-server --address 127.0.0.1 --port ${port} -D image-dir &
	lp_pid=$!

	# Restore lazily from checkpoint.
	# The restored container needs a different name as the checkpointed
	# container is not yet destroyed. It is only destroyed at that point
	# in time when the last page is lazily transferred to the destination.
	# Killing the CRIU on the checkpoint side will let the container
	# continue to run if the migration failed at some point.
	runc_restore_with_pipes ./image-dir test_busybox_restore --lazy-pages

	wait $cpt_pid

	wait $lp_pid

	check_pipes
}

@test "checkpoint and restore in external network namespace" {
	# check if external_net_ns is supported; only with criu 3.10++
	if ! "${CRIU}" check --feature external_net_ns; then
		# this criu does not support external_net_ns; skip the test
		skip "this criu does not support external network namespaces"
	fi

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

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	for _ in $(seq 2); do
		# checkpoint the running container; this automatically tells CRIU to
		# handle the network namespace defined in config.json as an external
		runc --criu "$CRIU" checkpoint --work-path ./work-dir test_busybox
		grep -B 5 Error ./work-dir/dump.log || true
		[ "$status" -eq 0 ]

		# after checkpoint busybox is no longer running
		testcontainer test_busybox checkpointed

		# restore from checkpoint; this should restore the container into the existing network namespace
		runc --criu "$CRIU" restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox
		grep -B 5 Error ./work-dir/restore.log || true
		[ "$status" -eq 0 ]

		# busybox should be back up and running
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
	# This is the file we write to /etc/criu/default.conf
	tmplog1=$(mktemp /tmp/runc-criu-log-XXXXXX.log)
	unlink "$tmplog1"
	tmplog1=$(basename "$tmplog1")
	# That is the actual configuration file to be used
	tmplog2=$(mktemp /tmp/runc-criu-log-XXXXXX.log)
	unlink "$tmplog2"
	tmplog2=$(basename "$tmplog2")
	# This adds the annotation 'org.criu.config' to set a container
	# specific CRIU config file.
	update_config '.annotations += {"org.criu.config": "'"$tmp"'"}'

	# Tell CRIU to use another configuration file
	mkdir -p /etc/criu
	echo "log-file=$tmplog1" >/etc/criu/default.conf
	# Make sure the RPC defined configuration file overwrites the previous
	echo "log-file=$tmplog2" >"$tmp"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	# checkpoint the running container
	runc --criu "$CRIU" checkpoint --work-path ./work-dir test_busybox
	grep -B 5 Error ./work-dir/dump.log || true
	[ "$status" -eq 0 ]
	! test -f ./work-dir/"$tmplog1"
	test -f ./work-dir/"$tmplog2"

	# after checkpoint busybox is no longer running
	testcontainer test_busybox checkpointed

	test -f ./work-dir/"$tmplog2" && unlink ./work-dir/"$tmplog2"
	# restore from checkpoint
	runc --criu "$CRIU" restore -d --work-path ./work-dir --console-socket "$CONSOLE_SOCKET" test_busybox
	grep -B 5 Error ./work-dir/restore.log || true
	[ "$status" -eq 0 ]
	! test -f ./work-dir/"$tmplog1"
	test -f ./work-dir/"$tmplog2"

	# busybox should be back up and running
	testcontainer test_busybox running
	unlink "$tmp"
	test -f ./work-dir/"$tmplog2" && unlink ./work-dir/"$tmplog2"
}
