#!/usr/bin/env bats

load helpers

function setup() {
	teardown_seccompagent
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
	update_config '.process.args = ["/bin/sh", "-c", "'"$1"'"] |
	.process.noNewPrivileges = '"$2"' |
	.linux.seccomp = {
		"defaultAction":"SCMP_ACT_ALLOW",
		"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
		"listenerMetadata": "bar",
		"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32"],
		"syscalls":[{"names":['"$3"'], "action":"SCMP_ACT_NOTIFY"}]
	}'
}

# Support for seccomp notify requires Linux > 5.6 because
# runc uses the pidfd_getfd system call to fetch the seccomp fd.
# https://github.com/torvalds/linux/commit/8649c322f75c96e7ced2fec201e123b2b073bf09

# The call to seccomp is done at different places according to the value of
# noNewPrivileges, for this reason many of the following cases are tested with
# both values.

# check that SCMP_ACT_NOTIFY on older kernels fails with a clear error message.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY old kernel)" {
	if [[ "$KERNEL_MAJOR" -gt 5 || ("$KERNEL_MAJOR" -eq 5 && "$KERNEL_MINOR" -ge 6) ]]; then
		skip "requires kernel less than 5.6"
	fi

	scmp_act_notify_template "/bin/true" false '"mkdir"'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges false)" {
	requires_kernel 5.6

	scmp_act_notify_template "mkdir /dev/shm/foo && stat /dev/shm/foo-bar" false '"mkdir"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges true)" {
	requires_kernel 5.6

	scmp_act_notify_template "mkdir /dev/shm/foo && stat /dev/shm/foo-bar" true '"mkdir"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc exec [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges false)" {
	requires_kernel 5.6
	requires root

	scmp_act_notify_template "sleep infinity" false '"mkdir"'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox /bin/sh -c "mkdir /dev/shm/foo && stat /dev/shm/foo-bar"
	[ "$status" -eq 0 ]
}

@test "runc exec [seccomp] (SCMP_ACT_NOTIFY noNewPrivileges true)" {
	requires_kernel 5.6
	requires root

	scmp_act_notify_template "sleep infinity" true '"mkdir"'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	runc exec test_busybox /bin/sh -c "mkdir /dev/shm/foo && stat /dev/shm/foo-bar"
	[ "$status" -eq 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY important syscalls noNewPrivileges false)" {
	requires_kernel 5.6

	scmp_act_notify_template "/bin/true" false '"execve","openat","open","read","close"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY important syscalls noNewPrivileges true)" {
	requires_kernel 5.6

	scmp_act_notify_template "/bin/true" true '"execve","openat","open","read","close"'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [seccomp] (empty listener path)" {
	update_config '.process.args = ["/bin/sh", "-c", "mkdir /dev/shm/foo && stat /dev/shm/foo"] |
		.linux.seccomp = {
			"defaultAction":"SCMP_ACT_ALLOW",
			"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
			"listenerMetadata": "bar",
		}'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY empty listener path)" {
	requires_kernel 5.6

	scmp_act_notify_template "/bin/true" false '"mkdir"'
	update_config '.linux.seccomp.listenerPath = ""'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY wrong listener path)" {
	requires_kernel 5.6

	scmp_act_notify_template "/bin/true" false '"mkdir"'
	update_config '.linux.seccomp.listenerPath = "/some-non-existing-listener-path.sock"'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

@test "runc run [seccomp] (SCMP_ACT_NOTIFY abstract listener path)" {
	requires_kernel 5.6

	scmp_act_notify_template "/bin/true" false '"mkdir"'
	update_config '.linux.seccomp.listenerPath = "@mysocketishere"'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

# Check that killing the seccompagent doesn't block syscalls in
# the container. They should return ENOSYS instead.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY kill seccompagent)" {
	requires_kernel 5.6

	scmp_act_notify_template "sleep 4 && mkdir /dev/shm/foo" false '"mkdir"'

	sleep 2 && teardown_seccompagent &
	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"Function not implemented"* ]]
}

# check that trying to use SCMP_ACT_NOTIFY with write() gives a meaningfull error.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY write)" {
	requires_kernel 5.6

	scmp_act_notify_template "/bin/true" false '"write"'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"SCMP_ACT_NOTIFY cannot be used for the write syscall"* ]]
}

# check that a startContainer hook doesn't get any extra file descriptor.
@test "runc run [seccomp] (SCMP_ACT_NOTIFY startContainer hook)" {
	requires_kernel 5.6

	# shellcheck disable=SC2016
	# For some (unknown) reason shellcheck doesn't like L199 together with the "if" in L210.
	update_config '.process.args = ["/bin/true"] |
		.linux.seccomp = {
			"defaultAction":"SCMP_ACT_ALLOW",
			"listenerPath": "'"$SECCCOMP_AGENT_SOCKET"'",
			"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32"],
			"syscalls":[{"names":["mkdir"], "action":"SCMP_ACT_NOTIFY"}]
		} |
		.hooks = {
			"startContainer": [
				{
					"path": "/bin/sh",
					"args": [
						"sh",
						"-c",
						"if [ $(ls /proc/self/fd/ | wc -l) -ne 4 ]; then echo \"File descriptors is not 4\". && ls /proc/self/fd/ | wc -l && exit 1; fi"
					],
				}
			]
		}'

	runc run test_busybox
	[ "$status" -eq 0 ]
}
