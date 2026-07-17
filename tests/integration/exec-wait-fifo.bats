#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# bats test_tags=runc-specific
@test "runc exec [ --exec-wait-fifo ] waits for a caller-relative FIFO" {
	update_config '.root.readonly = false'
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	local caller="$BATS_TEST_TMPDIR/caller"
	mkdir "$caller"
	local fifo="$caller/exec-wait.fifo"
	mkfifo "$fifo"

	# Give runc its own stdio: the parked exec process keeps it open, so bats'
	# run helper would hang waiting for EOF even after the detached parent exits.
	local log="$BATS_TEST_TMPDIR/exec-wait.log" ret=0
	(
		cd "$caller" || exit
		timeout -k 5 30 runc exec -d --pid-file exec.pid --exec-wait-fifo ./exec-wait.fifo test_busybox \
			sh -c "echo ran > /exec-wait.marker" </dev/null >"$log" 2>&1
	) || ret=$?
	run -0 cat "$log"

	[ "$ret" -eq 0 ]
	[ -s "$caller/exec.pid" ]
	local pid
	pid=$(cat "$caller/exec.pid")
	[[ "$pid" =~ ^[0-9]+$ ]]
	[ "$pid" -gt 0 ]
	kill -0 "$pid"

	# The program has not run: the exec process is blocked before execve.
	run -1 runc exec test_busybox test -e /exec-wait.marker

	# Consume the acknowledgment before closing the reader, avoiding EPIPE.
	run -0 timeout 10 cat "$fifo"
	assert_output "0"

	retry 10 1 runc exec test_busybox test -e /exec-wait.marker
	[ -p "$fifo" ]
}

# bats test_tags=runc-specific
@test "runc exec [ --exec-wait-fifo ] rejects a non-FIFO path" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	local not_fifo="$PWD/not-a-fifo"
	: >"$not_fifo"

	run -255 runc exec --exec-wait-fifo "$not_fifo" test_busybox true
	assert_output --partial "is not a fifo"
}

# bats test_tags=runc-specific
@test "runc exec [ --exec-wait-fifo ] rejects a missing FIFO" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running

	run -255 runc exec --exec-wait-fifo "$PWD/does-not-exist.fifo" test_busybox true
	assert_output --partial "failed to open exec wait fifo"
}
