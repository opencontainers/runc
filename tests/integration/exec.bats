#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	# Remove the sibling cgroup created by "runc exec --cgroup" tests, if any.
	# It has to be removed before teardown_bundle, as the latter removes the
	# parent cgroup, which can't be done while it has any children.
	if [ -v SIBLING_CGROUP ]; then
		rmdir "$SIBLING_CGROUP"
		unset SIBLING_CGROUP
	fi
	teardown_bundle
}

@test "runc exec" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox echo Hello from exec
	echo text echoed = "'""${output}""'"
	[[ "${output}" == *"Hello from exec"* ]]
}

@test "runc exec [exit codes]" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -1 runc exec test_busybox false

	run -42 runc exec test_busybox sh -c "exit 42"

	run -255 runc exec --pid-file /non-existent/directory test_busybox true

	run -255 runc exec test_busybox no-such-binary

	run -255 runc exec no_such_container true
}

@test "runc exec --pid-file" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec --pid-file pid.txt test_busybox echo Hello from exec
	echo text echoed = "'""${output}""'"
	[[ "${output}" == *"Hello from exec"* ]]

	[ -e pid.txt ]
	output=$(cat pid.txt)
	[[ "$output" =~ [0-9]+ ]]
	[[ "$output" != $(runc state test_busybox | jq '.pid') ]]
}

@test "runc exec --pid-file with new CWD" {
	bundle="$(pwd)"
	mkdir pid_file
	cd pid_file

	run -0 runc run -d -b "$bundle" --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec --pid-file pid.txt test_busybox echo Hello from exec
	echo text echoed = "'""${output}""'"
	[[ "${output}" == *"Hello from exec"* ]]

	[ -e pid.txt ]
	output=$(cat pid.txt)
	[[ "$output" =~ [0-9]+ ]]
	[[ "$output" != $(runc state test_busybox | jq '.pid') ]]
}

@test "runc exec ls -la" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox ls -la
	[[ ${lines[0]} == *"total"* ]]
	[[ ${lines[1]} == *"."* ]]
	[[ ${lines[2]} == *".."* ]]
}

@test "runc exec ls -la with --cwd" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec --cwd /bin test_busybox pwd
	[[ ${output} == "/bin"* ]]
}

@test "runc exec --env" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec --env RUNC_EXEC_TEST=true test_busybox env
	[[ "$output" == *"RUNC_EXEC_TEST=true"* ]]
	# Env should inherit what's in config.json.
	[[ "$output" == *'PATH='* ]]
	[[ "$output" == *'TERM='* ]]
	# Env should have HOME set from container's /etc/passwd.
	[[ "$output" == *'HOME='* ]]
}

@test "runc exec --user" {
	# --user can't work in rootless containers that don't have idmap.
	[ $EUID -ne 0 ] && requires rootless_idmap

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec --user 1000:1000 test_busybox id
	[[ "${output}" == "uid=1000 gid=1000"* ]]

	# Check the default value of HOME ("/") is set even in case when
	#  - HOME is not set in process.Env, and
	#  - there is no entry in container's /etc/passwd for a given UID.
	#
	# NOTE this is not a standard runtime feature, but rather
	# a historical de facto behavior we're afraid to change.

	# shellcheck disable=SC2016
	run -0 runc exec --user 1000 test_busybox sh -u -c 'echo $HOME'
	[[ "$output" = "/" ]]
}

# https://github.com/opencontainers/runc/issues/3674.
@test "runc exec --user vs /dev/null ownership" {
	requires root

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	ls -l /dev/null
	runc exec -d --user 1000:1000 test_busybox id </dev/null
	ls -l /dev/null
	UG=$(stat -c %u:%g /dev/null)

	# Host's /dev/null must be owned by root.
	[ "$UG" = "0:0" ]
}

@test "runc exec --additional-gids" {
	requires root

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	wait_for_container 15 1 test_busybox

	run -0 runc exec --user 1000:1000 --additional-gids 100 --additional-gids 65534 test_busybox id -G
	[ "$output" = "1000 100 65534" ]
}

@test "runc exec --preserve-fds" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	echo hello >preserve-fds.test
	# fd 3 is used by bats, so we use 4
	exec 4<preserve-fds.test
	run -0 runc exec --preserve-fds=2 test_busybox cat /proc/self/fd/4
	[ "${output}" = "hello" ]
}

function check_exec_debug() {
	[[ "$*" == *"nsexec container setup"* ]]
	[[ "$*" == *"child process in init()"* ]]
	[[ "$*" == *"setns_init: about to exec"* ]]
}

@test "runc --debug exec" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test

	run -0 runc --debug exec test true
	[[ "${output}" == *"level=debug"* ]]
	check_exec_debug "$output"
}

@test "runc --debug --log exec" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test

	run runc --debug --log log.out exec test true
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

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	# Check we can't join parent cgroup.
	run ! runc exec --cgroup ".." test_busybox cat /proc/self/cgroup
	[[ "$output" == *"bad sub cgroup path"* ]]

	# Check we can't join a sibling cgroup sharing our name prefix.
	local cpu_path sibling
	cpu_path=$(get_cgroup_path cpu)
	sibling="$(basename "$cpu_path")-sibling"
	# SIBLING_CGROUP is removed by teardown.
	SIBLING_CGROUP="$(dirname "$cpu_path")/$sibling"
	mkdir "$SIBLING_CGROUP"
	run ! runc exec --cgroup "../$sibling" test_busybox cat /proc/self/cgroup
	[[ "$output" == *"bad sub cgroup path"* ]]

	# Same as above, for a particular controller.
	run ! runc exec --cgroup "cpu:../$sibling" test_busybox cat /proc/self/cgroup
	[[ "$output" == *"bad sub cgroup path"* ]]

	# Check we can't join non-existing subcgroup.
	run ! runc exec --cgroup nonexistent test_busybox cat /proc/self/cgroup
	[[ "$output" == *" adding pid "*"o such file or directory"* ]]

	# Check we can't join non-existing subcgroup (for a particular controller).
	run ! runc exec --cgroup cpu:nonexistent test_busybox cat /proc/self/cgroup
	[[ "$output" == *" adding pid "*"o such file or directory"* ]]

	# Check we can't specify non-existent controller.
	run ! runc exec --cgroup whaaat:/ test_busybox true
	[[ "$output" == *"unknown controller "* ]]

	# Check we can join top-level cgroup (implicit).
	run -0 runc exec test_busybox cat /proc/self/cgroup
	run ! grep -v ":$REL_CGROUPS_PATH\$" <<<"$output"

	# Check we can join top-level cgroup (explicit).
	run -0 runc exec --cgroup / test_busybox cat /proc/self/cgroup
	run ! grep -v ":$REL_CGROUPS_PATH\$" <<<"$output"

	# Create a few subcgroups.
	# Note that cpu,cpuacct may be mounted together or separate.
	run -0 runc exec test_busybox sh -euc "mkdir -p /sys/fs/cgroup/memory/submem /sys/fs/cgroup/cpu/subcpu /sys/fs/cgroup/cpuacct/subcpu"

	# Check that explicit --cgroup works.
	run -0 runc exec --cgroup memory:submem --cgroup cpu,cpuacct:subcpu test_busybox cat /proc/self/cgroup
	[[ "$output" == *":memory:$REL_CGROUPS_PATH/submem"* ]]
	[[ "$output" == *":cpu"*":$REL_CGROUPS_PATH/subcpu"* ]]
}

@test "runc exec --cgroup subcgroup [v2]" {
	requires root cgroups_v2

	set_cgroups_path
	set_cgroup_mount_writable

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	# Check we can't join parent cgroup.
	run ! runc exec --cgroup ".." test_busybox cat /proc/self/cgroup
	[[ "$output" == *"bad sub cgroup path"* ]]

	# Check we can't join a sibling cgroup sharing our name prefix.
	local sibling
	sibling="$(basename "$CGROUP_V2_PATH")-sibling"
	# SIBLING_CGROUP is removed by teardown.
	SIBLING_CGROUP="$(dirname "$CGROUP_V2_PATH")/$sibling"
	mkdir "$SIBLING_CGROUP"
	run ! runc exec --cgroup "../$sibling" test_busybox cat /proc/self/cgroup
	[[ "$output" == *"bad sub cgroup path"* ]]

	# Check we can't join non-existing subcgroup.
	run ! runc exec --cgroup nonexistent test_busybox cat /proc/self/cgroup
	[[ "$output" == *" cgroup"*"o such file or directory"* ]]

	# Check we can join top-level cgroup (implicit).
	run -0 runc exec test_busybox grep '^0::/$' /proc/self/cgroup

	# Check we can join top-level cgroup (explicit).
	run -0 runc exec --cgroup / test_busybox grep '^0::/$' /proc/self/cgroup

	# Now move "init" to a subcgroup, and check it was moved.
	run -0 runc exec test_busybox sh -euc "mkdir /sys/fs/cgroup/foobar \
		&& echo 1 > /sys/fs/cgroup/foobar/cgroup.procs \
		&& grep -w foobar /proc/1/cgroup"

	# The following part is taken from
	# @test "runc exec (cgroup v2 + init process in non-root cgroup) succeeds"

	# The init process is now in "/foo", but an exec process can still
	# join "/" because we haven't enabled any domain controller yet.
	run -0 runc exec test_busybox grep '^0::/$' /proc/self/cgroup

	# Turn on a domain controller (memory).
	run -0 runc exec test_busybox sh -euc 'echo $$ > /sys/fs/cgroup/foobar/cgroup.procs; echo +memory > /sys/fs/cgroup/cgroup.subtree_control'

	# An exec process can no longer join "/" after turning on a domain
	# controller.  Check that cgroup v2 fallback to init cgroup works.
	run -0 runc exec test_busybox sh -euc "cat /proc/self/cgroup && grep '^0::/foobar$' /proc/self/cgroup"

	# Check that --cgroup / disables the init cgroup fallback.
	run ! runc exec --cgroup / test_busybox true
	[[ "$output" == *" adding pid "*" to cgroups"*"evice or resource busy"* ]]

	# Check that explicit --cgroup foobar works.
	run -0 runc exec --cgroup foobar test_busybox grep '^0::/foobar$' /proc/self/cgroup

	# Check all processes is in foobar (this check is redundant).
	run -0 runc exec --cgroup foobar test_busybox sh -euc '! grep -vwH foobar /proc/*/cgroup'

	# Add a second subcgroup, check we're in it.
	run -0 runc exec --cgroup foobar test_busybox mkdir /sys/fs/cgroup/second
	run -0 runc exec --cgroup second test_busybox grep -w second /proc/self/cgroup
}

# https://github.com/opencontainers/runc/issues/5089
@test "runc exec [init changes cgroup]" {
	requires root cgroups_v2

	NEW_CGROUP_REL=/runc-tst-$$
	NEW_CGROUP=/sys/fs/cgroup$NEW_CGROUP_REL
	mkdir $NEW_CGROUP

	# The container is placed into a $CGROUP_V2_PATH cgroup.
	set_cgroups_path
	# And upon the start it moves itself into $NEW_CGROUP.
	set_cgroup_mount_writable
	update_config '	  .linux.namespaces -= [{"type": "cgroup"}]
			| .process.args = ["sh", "-c", "echo 1 > '$NEW_CGROUP'/cgroup.procs && exec sleep 1h"]'

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running
	sleep 1
	# Remove the original container cgroup. If systemd cgroup manager is used by runc,
	# the cgroup might have already be deleted by systemd, so we ignore rmdir errors.
	rmdir "$CGROUP_V2_PATH" || true
	test -d "$CGROUP_V2_PATH" && false

	# Test that runc exec is able to fallback to container's init cgroup
	# even if the original cgroup is gone.
	run -0 runc exec test_busybox cat /proc/self/cgroup
	[ "$output" = "0::$NEW_CGROUP_REL" ]

	# Cleanup.
	run runc delete -f test_busybox
	rmdir "$NEW_CGROUP"
}

@test "runc exec [execve error]" {
	cat <<EOF >rootfs/run.sh
#!/mmnnttbb foo bar
sh
EOF
	chmod +x rootfs/run.sh
	run runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	run ! runc exec -t test_busybox /run.sh

	# After the sync socket closed, we should not send error to parent
	# process, or else we will get a unnecessary error log(#4171).
	# Although we never close the sync socket when doing exec,
	# but we need to keep this test to ensure this behavior is always right.
	[ ${#lines[@]} -eq 1 ]
	[[ ${lines[0]} = *"exec /run.sh: no such file or directory"* ]]
}

# https://github.com/opencontainers/runc/issues/4688
@test "runc exec check default home" {
	# --user can't work in rootless containers that don't have idmap.
	[ $EUID -ne 0 ] && requires rootless_idmap
	echo 'tempuser:x:2000:2000:tempuser:/home/tempuser:/bin/sh' >>rootfs/etc/passwd

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test

	run -0 runc exec -u 2000 test sh -c "echo \$HOME"
	[ "${lines[0]}" = "/home/tempuser" ]
}
