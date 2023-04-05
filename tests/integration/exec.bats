#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc exec" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox echo Hello from exec
	[ "$status" -eq 0 ]
	echo text echoed = "'""${output}""'"
	[[ "${output}" == *"Hello from exec"* ]]
}

@test "runc exec [exit codes]" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox false
	[ "$status" -eq 1 ]

	runc exec test_busybox sh -c "exit 42"
	[ "$status" -eq 42 ]

	runc exec --pid-file /non-existent/directory test_busybox true
	[ "$status" -eq 255 ]

	runc exec test_busybox no-such-binary
	[ "$status" -eq 255 ]

	runc exec no_such_container true
	[ "$status" -eq 255 ]
}

@test "runc exec --pid-file" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --pid-file pid.txt test_busybox echo Hello from exec
	[ "$status" -eq 0 ]
	echo text echoed = "'""${output}""'"
	[[ "${output}" == *"Hello from exec"* ]]

	# check pid.txt was generated
	[ -e pid.txt ]

	output=$(cat pid.txt)
	[[ "$output" =~ [0-9]+ ]]
	[[ "$output" != $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc exec --pid-file with new CWD" {
	bundle="$(pwd)"
	# create pid_file directory as the CWD
	mkdir pid_file
	cd pid_file

	# run busybox detached
	runc run -d -b "$bundle" --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --pid-file pid.txt test_busybox echo Hello from exec
	[ "$status" -eq 0 ]
	echo text echoed = "'""${output}""'"
	[[ "${output}" == *"Hello from exec"* ]]

	# check pid.txt was generated
	[ -e pid.txt ]

	output=$(cat pid.txt)
	[[ "$output" =~ [0-9]+ ]]
	[[ "$output" != $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc exec ls -la" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox ls -la
	[ "$status" -eq 0 ]
	[[ ${lines[0]} == *"total"* ]]
	[[ ${lines[1]} == *"."* ]]
	[[ ${lines[2]} == *".."* ]]
}

@test "runc exec ls -la with --cwd" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --cwd /bin test_busybox pwd
	[ "$status" -eq 0 ]
	[[ ${output} == "/bin"* ]]
}

@test "runc exec --env" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --env RUNC_EXEC_TEST=true test_busybox env
	[ "$status" -eq 0 ]

	[[ ${output} == *"RUNC_EXEC_TEST=true"* ]]
}

@test "runc exec --user" {
	# --user can't work in rootless containers that don't have idmap.
	[ $EUID -ne 0 ] && requires rootless_idmap

	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --user 1000:1000 test_busybox id
	[ "$status" -eq 0 ]
	[[ "${output}" == "uid=1000 gid=1000"* ]]
}

# https://github.com/opencontainers/runc/issues/3674.
@test "runc exec --user vs /dev/null ownership" {
	requires root

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	ls -l /dev/null
	__runc exec -d --user 1000:1000 test_busybox id </dev/null
	ls -l /dev/null
	UG=$(stat -c %u:%g /dev/null)

	# Host's /dev/null must be owned by root.
	[ "$UG" = "0:0" ]
}

@test "runc exec --additional-gids" {
	requires root

	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	wait_for_container 15 1 test_busybox

	runc exec --user 1000:1000 --additional-gids 100 --additional-gids 65534 test_busybox id -G
	[ "$status" -eq 0 ]
	[ "$output" = "1000 100 65534" ]
}

@test "runc exec --preserve-fds" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	echo hello >preserve-fds.test
	# fd 3 is used by bats, so we use 4
	exec 4<preserve-fds.test
	runc exec --preserve-fds=2 test_busybox cat /proc/self/fd/4
	[ "$status" -eq 0 ]
	[ "${output}" = "hello" ]
}

function check_exec_debug() {
	[[ "$*" == *"nsexec container setup"* ]]
	[[ "$*" == *"child process in init()"* ]]
	[[ "$*" == *"setns_init: about to exec"* ]]
}

@test "runc --debug exec" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test
	[ "$status" -eq 0 ]

	runc --debug exec test true
	[ "$status" -eq 0 ]
	[[ "${output}" == *"level=debug"* ]]
	check_exec_debug "$output"
}

@test "runc --debug --log exec" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test
	[ "$status" -eq 0 ]

	runc --debug --log log.out exec test true
	# check output does not include debug info
	[[ "${output}" != *"level=debug"* ]]

	cat log.out >&2
	# check expected debug output was sent to log.out
	output=$(cat log.out)
	[[ "${output}" == *"level=debug"* ]]
	check_exec_debug "$output"
}

@test "runc exec --cgroup sub-cgroups [v1]" {
	requires root cgroups_v1

	set_cgroups_path
	set_cgroup_mount_writable

	__runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	# Check we can't join parent cgroup.
	runc exec --cgroup ".." test_busybox cat /proc/self/cgroup
	[ "$status" -ne 0 ]
	[[ "$output" == *" .. is not a sub cgroup path"* ]]

	# Check we can't join non-existing subcgroup.
	runc exec --cgroup nonexistent test_busybox cat /proc/self/cgroup
	[ "$status" -ne 0 ]
	[[ "$output" == *" adding pid "*"/nonexistent/cgroup.procs: no such file "* ]]

	# Check we can't join non-existing subcgroup (for a particular controller).
	runc exec --cgroup cpu:nonexistent test_busybox cat /proc/self/cgroup
	[ "$status" -ne 0 ]
	[[ "$output" == *" adding pid "*"/nonexistent/cgroup.procs: no such file "* ]]

	# Check we can't specify non-existent controller.
	runc exec --cgroup whaaat:/ test_busybox true
	[ "$status" -ne 0 ]
	[[ "$output" == *"unknown controller "* ]]

	# Check we can join top-level cgroup (implicit).
	runc exec test_busybox cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	run ! grep -v ":$REL_CGROUPS_PATH\$" <<<"$output"

	# Check we can join top-level cgroup (explicit).
	runc exec --cgroup / test_busybox cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	run ! grep -v ":$REL_CGROUPS_PATH\$" <<<"$output"

	# Create a few subcgroups.
	# Note that cpu,cpuacct may be mounted together or separate.
	runc exec test_busybox sh -euc "mkdir -p /sys/fs/cgroup/memory/submem /sys/fs/cgroup/cpu/subcpu /sys/fs/cgroup/cpuacct/subcpu"
	[ "$status" -eq 0 ]

	# Check that explicit --cgroup works.
	runc exec --cgroup memory:submem --cgroup cpu,cpuacct:subcpu test_busybox cat /proc/self/cgroup
	[ "$status" -eq 0 ]
	[[ "$output" == *":memory:$REL_CGROUPS_PATH/submem"* ]]
	[[ "$output" == *":cpu"*":$REL_CGROUPS_PATH/subcpu"* ]]
}

@test "runc exec --cgroup subcgroup [v2]" {
	requires root cgroups_v2

	set_cgroups_path
	set_cgroup_mount_writable

	__runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	# Check we can't join parent cgroup.
	runc exec --cgroup ".." test_busybox cat /proc/self/cgroup
	[ "$status" -ne 0 ]
	[[ "$output" == *" .. is not a sub cgroup path"* ]]

	# Check we can't join non-existing subcgroup.
	runc exec --cgroup nonexistent test_busybox cat /proc/self/cgroup
	[ "$status" -ne 0 ]
	[[ "$output" == *" adding pid "*"/nonexistent/cgroup.procs: no such file "* ]]

	# Check we can join top-level cgroup (implicit).
	runc exec test_busybox grep '^0::/$' /proc/self/cgroup
	[ "$status" -eq 0 ]

	# Check we can join top-level cgroup (explicit).
	runc exec --cgroup / test_busybox grep '^0::/$' /proc/self/cgroup
	[ "$status" -eq 0 ]

	# Now move "init" to a subcgroup, and check it was moved.
	runc exec test_busybox sh -euc "mkdir /sys/fs/cgroup/foobar \
		&& echo 1 > /sys/fs/cgroup/foobar/cgroup.procs \
		&& grep -w foobar /proc/1/cgroup"
	[ "$status" -eq 0 ]

	# The following part is taken from
	# @test "runc exec (cgroup v2 + init process in non-root cgroup) succeeds"

	# The init process is now in "/foo", but an exec process can still
	# join "/" because we haven't enabled any domain controller yet.
	runc exec test_busybox grep '^0::/$' /proc/self/cgroup
	[ "$status" -eq 0 ]

	# Turn on a domain controller (memory).
	runc exec test_busybox sh -euc 'echo $$ > /sys/fs/cgroup/foobar/cgroup.procs; echo +memory > /sys/fs/cgroup/cgroup.subtree_control'
	[ "$status" -eq 0 ]

	# An exec process can no longer join "/" after turning on a domain
	# controller.  Check that cgroup v2 fallback to init cgroup works.
	runc exec test_busybox sh -euc "cat /proc/self/cgroup && grep '^0::/foobar$' /proc/self/cgroup"
	[ "$status" -eq 0 ]

	# Check that --cgroup / disables the init cgroup fallback.
	runc exec --cgroup / test_busybox true
	[ "$status" -ne 0 ]
	[[ "$output" == *" adding pid "*" to cgroups"*"/cgroup.procs: device or resource busy"* ]]

	# Check that explicit --cgroup foobar works.
	runc exec --cgroup foobar test_busybox grep '^0::/foobar$' /proc/self/cgroup
	[ "$status" -eq 0 ]

	# Check all processes is in foobar (this check is redundant).
	runc exec --cgroup foobar test_busybox sh -euc '! grep -vwH foobar /proc/*/cgroup'
	[ "$status" -eq 0 ]

	# Add a second subcgroup, check we're in it.
	runc exec --cgroup foobar test_busybox mkdir /sys/fs/cgroup/second
	[ "$status" -eq 0 ]
	runc exec --cgroup second test_busybox grep -w second /proc/self/cgroup
	[ "$status" -eq 0 ]
}
