#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc exec [ --exec-wait-fifo ] does not run the program until the read end is opened" {
	update_config '.root.readonly = false'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	testcontainer test_busybox running

	local fifo="$PWD/exec-wait.fifo"
	mkfifo "$fifo"

	# Use __runc, not the run() helper: the exec process blocks before execve
	# while holding the inherited stdio, so the run() helper would wait forever
	# for those fds to reach EOF. The detached parent still writes the pid file
	# and exits.
	__runc exec -d --pid-file exec.pid --exec-wait-fifo "$fifo" test_busybox \
		sh -c "echo ran > /exec-wait.marker" </dev/null
	[ -f exec.pid ]

	# The program has not run: the exec process is blocked before execve.
	runc exec test_busybox test -e /exec-wait.marker
	[ "$status" -ne 0 ]

	# Opening the read end unblocks the process, which then runs its program.
	timeout 10 cat "$fifo" >/dev/null

	retry 10 1 __runc exec test_busybox test -e /exec-wait.marker
}

@test "runc exec [ --exec-wait-fifo ] rejects a path that is not a fifo" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	testcontainer test_busybox running

	local not_fifo="$PWD/not-a-fifo"
	: >"$not_fifo"

	runc exec --exec-wait-fifo "$not_fifo" test_busybox true
	[ "$status" -ne 0 ]
	[[ "$output" == *"is not a fifo"* ]]
}

@test "runc exec [ --exec-wait-fifo ] fails for a missing fifo path" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	testcontainer test_busybox running

	runc exec --exec-wait-fifo "$PWD/does-not-exist.fifo" test_busybox true
	[ "$status" -ne 0 ]
	[[ "$output" == *"failed to open exec wait fifo"* ]]
}
