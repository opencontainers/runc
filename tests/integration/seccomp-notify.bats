#!/usr/bin/env bats

load helpers

# Support for seccomp notify requires Linux > 5.6 because
# runc uses the pidfd_getfd system call to fetch the seccomp fd.
# https://github.com/torvalds/linux/commit/8649c322f75c96e7ced2fec201e123b2b073bf09
# We also require arch x86_64, to not make this fail when people run tests
# locally on other archs.
function setup() {
	requires_kernel 5.6
	requires arch_x86_64

	setup_seccompagent
	setup_busybox
}

function teardown() {
	teardown_seccompagent
	teardown_bundle
}

# Create config.json template with SCMP_ACT_NOTIFY actions
# $1: command to run
# $2: noNewPrivileges (false/true)
# $3: list of syscalls
function scmp_act_notify_template() {
	# The agent intercepts mkdir syscalls and creates the folder appending
	# "-bar" (listenerMetadata below) to the name.
	update_config '   .process.args = ["/bin/sh", "-c", "'"$1"'"]
			| .process.noNewPrivileges = '"$2"'
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
				"listenerMetadata": "bar",
				"architectures": [ "SCMP_ARCH_X86","SCMP_ARCH_X32", "SCMP_ARCH_X86_64" ],
				"syscalls": [{ "names": ['"$3"'], "action": "SCMP_ACT_NOTIFY" }]
			}'
}

# The call to seccomp is done at different places according to the value of
# noNewPrivileges, for this reason many of the following cases are tested with
# both values.

# Test basic actions handled by the agent work fine. noNewPrivileges FALSE.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges false)" {
	scmp_act_notify_template "mkdir /dev/shm/foo && stat /dev/shm/foo-bar" false '"mkdir"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Test basic actions handled by the agent work fine. noNewPrivileges TRUE.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges true)" {
	scmp_act_notify_template "mkdir /dev/shm/foo && stat /dev/shm/foo-bar" true '"mkdir"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Test actions not-handled by the agent work fine. noNewPrivileges FALSE.
@test "runc exec [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges false)" {
	requires root

	scmp_act_notify_template "sleep infinity" false '"mkdir"'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox /bin/sh -c "mkdir /dev/shm/foo && stat /dev/shm/foo-bar"
	[ "$status" -eq 0 ]
}

# Test actions not-handled by the agent work fine. noNewPrivileges TRUE.
@test "runc exec [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges true)" {
	requires root

	scmp_act_notify_template "sleep infinity" true '"mkdir"'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	runc exec test_busybox /bin/sh -c "mkdir /dev/shm/foo && stat /dev/shm/foo-bar"
	[ "$status" -eq 0 ]
}

# Test important syscalls (some might be executed by runc) work fine when handled by the agent. noNewPrivileges FALSE.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY important syscalls noNewPrivileges false)" {
	scmp_act_notify_template "/bin/true" false '"execve","openat","open","read","close"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Test important syscalls (some might be executed by runc) work fine when handled by the agent. noNewPrivileges TRUE.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY important syscalls noNewPrivileges true)" {
	scmp_act_notify_template "/bin/true" true '"execve","openat","open","read","close"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Ignore listenerPath if the profile doesn't use seccomp notify actions.
@test "runc run [seccomp] (ignore listener path if no notify act)" {
	update_config '   .process.args = ["/bin/sh", "-c", "mkdir /dev/shm/foo && stat /dev/shm/foo"]
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
				"listenerMetadata": "bar",
			}'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Ensure listenerPath is present if the profile uses seccomp notify actions.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY empty listener path and notify act)" {
	scmp_act_notify_template "/bin/true" false '"mkdir"'
	update_config '.linux.seccomp.listenerPath = ""'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

# Test using an invalid socket (none listening) as listenerPath fails.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY wrong listener path)" {
	scmp_act_notify_template "/bin/true" false '"mkdir"'
	update_config '.linux.seccomp.listenerPath = "/some-non-existing-listener-path.sock"'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

# Test using an invalid abstract socket as listenerPath fails.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY wrong abstract listener path)" {
	scmp_act_notify_template "/bin/true" false '"mkdir"'
	update_config '.linux.seccomp.listenerPath = "@mysocketishere"'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

# Check that killing the seccompagent doesn't block syscalls in
# the container. They should return ENOSYS instead.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY kill seccompagent)" {
	scmp_act_notify_template "sleep 4 && mkdir /dev/shm/foo" false '"mkdir"'

	sleep 2 && teardown_seccompagent &
	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"mkdir:"*"/dev/shm/foo"*"Function not implemented"* ]]
}

# Check that starting with no seccomp agent running fails with a clear error.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY no seccompagent)" {
	teardown_seccompagent

	scmp_act_notify_template "/bin/true" false '"mkdir"'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"failed to connect with seccomp agent"* ]]
}

# Check that agent-returned error for the syscall works.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY error chmod)" {
	scmp_act_notify_template "touch /dev/shm/foo && chmod 777 /dev/shm/foo" false '"chmod", "fchmod", "fchmodat"'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"chmod:"*"/dev/shm/foo"*"No medium found"* ]]
}

# check that trying to use SCMP_ACT_NOTIFY with write() gives a meaningful error.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY write)" {
	scmp_act_notify_template "/bin/true" false '"write"'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"SCMP_ACT_NOTIFY cannot be used for the write syscall"* ]]
}

# check that a startContainer hook doesn't get any extra file descriptor.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY startContainer hook)" {
	# shellcheck disable=SC2016
	# We use single quotes to properly delimit the $1 param to
	# update_config(), but this shellshcheck is quite silly and fails if the
	# multi-line string includes some $var (even when it is properly outside of the
	# single quotes) or when we use this syntax to execute commands in the
	# string: $(command).
	# So, just disable this check for our usage of update_config().
	update_config '   .process.args = ["/bin/true"]
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
				"architectures": [ "SCMP_ARCH_X86", "SCMP_ARCH_X32", "SCMP_ARCH_X86_64" ],
				"syscalls":[{ "names": [ "mkdir" ], "action": "SCMP_ACT_NOTIFY" }]
			}
			|.hooks = {
				"startContainer": [ {
						"path": "/bin/sh",
						"args": [
							"sh",
							"-c",
							"if [ $(ls /proc/self/fd/ | wc -l) -ne 4 ]; then echo \"File descriptors is not 4\". && ls /proc/self/fd/ | wc -l && exit 1; fi"
						],
				} ]
			}'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# Check that example config in the seccomp agent dir works.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY example config)" {
	# Run the script used in the seccomp agent example.
	# This takes a bare config.json and modifies it to run an example.
	"${INTEGRATION_ROOT}/../../contrib/cmd/seccompagent/gen-seccomp-example-cfg.sh"

	# The listenerPath the previous command uses is the default used by the
	# seccomp agent. However, inside bats the socket is in a bats tmp dir.
	update_config '.linux.seccomp.listenerPath = "'"$SECCCOMP_AGENT_SOCKET"'"'

	runc run test_busybox

	[ "$status" -eq 0 ]
	[[ "$output" == *"chmod:"*"test-file"*"No medium found"* ]]
}
