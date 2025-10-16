#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run [stdin not a tty]" {
	# stty size fails without a tty
	update_config '(.. | select(.[]? == "sh")) += ["-c", "stty size"]'
	# note that stdout/stderr are already redirected by bats' run
	runc -0 run test_busybox </dev/null
}

@test "runc run [tty ptsname]" {
	# Replace sh script with readlink.
	# shellcheck disable=SC2016
	update_config '(.. | select(.[]? == "sh")) += ["-c", "for file in /proc/self/fd/[012]; do readlink $file; done"]'

	runc -0 run test_busybox
	[[ ${lines[0]} =~ /dev/pts/+ ]]
	[[ ${lines[1]} =~ /dev/pts/+ ]]
	[[ ${lines[2]} =~ /dev/pts/+ ]]
}

@test "runc run [tty owner]" {
	# tty chmod is not doable in rootless containers without idmap.
	# TODO: this can be made as a change to the gid test.
	[ $EUID -ne 0 ] && requires rootless_idmap

	# Replace sh script with stat.
	# shellcheck disable=SC2016
	update_config '(.. | select(.[]? == "sh")) += ["-c", "stat -c %u:%g $(tty) | tr : \\\\n"]'

	runc -0 run test_busybox
	[[ ${lines[0]} =~ 0 ]]
	# This is set by the default config.json (it corresponds to the standard tty group).
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc run [tty owner] ({u,g}id != 0)" {
	# tty chmod is not doable in rootless containers without idmap.
	[ $EUID -ne 0 ] && requires rootless_idmap

	# replace "uid": 0 with "uid": 1000
	# and do a similar thing for gid.
	# Replace sh script with stat.
	# shellcheck disable=SC2016
	update_config ' (.. | select(.uid? == 0)) .uid |= 1000
			| (.. | select(.gid? == 0)) .gid |= 100
			| (.. | select(.[]? == "sh")) += ["-c", "stat -c %u:%g $(tty) | tr : \\\\n"]'

	runc -0 run test_busybox
	[[ ${lines[0]} =~ 1000 ]]
	# This is set by the default config.json (it corresponds to the standard tty group).
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc exec [stdin not a tty]" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	# note that stdout/stderr are already redirected by bats' run
	runc -0 exec -t test_busybox sh -c "stty size" </dev/null
}

@test "runc exec [tty ptsname]" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	# shellcheck disable=SC2016
	runc -0 exec -t test_busybox sh -c 'for file in /proc/self/fd/[012]; do readlink $file; done'
	[[ ${lines[0]} =~ /dev/pts/+ ]]
	[[ ${lines[1]} =~ /dev/pts/+ ]]
	[[ ${lines[2]} =~ /dev/pts/+ ]]
}

@test "runc exec [tty owner]" {
	# tty chmod is not doable in rootless containers without idmap.
	# TODO: this can be made as a change to the gid test.
	[ $EUID -ne 0 ] && requires rootless_idmap

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	# shellcheck disable=SC2016
	runc -0 exec -t test_busybox sh -c 'stat -c %u:%g $(tty) | tr : \\n'
	[[ ${lines[0]} =~ 0 ]]
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc exec [tty owner] ({u,g}id != 0)" {
	# tty chmod is not doable in rootless containers without idmap.
	[ $EUID -ne 0 ] && requires rootless_idmap

	# replace "uid": 0 with "uid": 1000
	# and do a similar thing for gid.
	update_config ' (.. | select(.uid? == 0)) .uid |= 1000
			| (.. | select(.gid? == 0)) .gid |= 100'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	# shellcheck disable=SC2016
	runc -0 exec -t test_busybox sh -c 'stat -c %u:%g $(tty) | tr : \\n'
	[[ ${lines[0]} =~ 1000 ]]
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc exec [tty consolesize]" {
	# allow writing to filesystem
	update_config '(.. | select(.readonly? != null)) .readonly |= false'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	tty_info_with_consize_size='
{
    "terminal": true,
    "consoleSize": {
	    "height": 10,
	    "width": 110
    },
    "args": [
	    "/bin/sh",
	    "-c",
	    "/bin/stty -a > /tmp/tty-info"
    ],
    "cwd": "/"
}'

	# Run the detached exec.
	runc -0 exec -t --pid-file pid.txt -d --console-socket "$CONSOLE_SOCKET" -p <(echo "$tty_info_with_consize_size") test_busybox
	[ -e pid.txt ]

	# Wait for the exec to finish.
	wait_pids_gone 100 0.5 "$(cat pid.txt)"

	tty_info='
{
    "args": [
	"/bin/cat",
	"/tmp/tty-info"
    ],
    "cwd": "/"
}'

	runc -0 exec -t -p <(echo "$tty_info") test_busybox

	# test tty width and height against original process.json
	[[ ${lines[0]} =~ "rows 10; columns 110" ]]
}

@test "runc create [terminal=false]" {
	# Disable terminal creation.
	# Replace sh script with sleep.
	update_config ' (.. | select(.terminal? != null)) .terminal |= false
			| (.. | select(.[]? == "sh")) += ["sleep", "1000s"]
			| del(.. | select(.? == "sh"))'

	# Make sure that the handling of detached IO is done properly. See #1354.
	__runc create test_busybox

	runc -0 start test_busybox

	testcontainer test_busybox running

	runc -0 kill test_busybox KILL
}

@test "runc run [terminal=false]" {
	# Disable terminal creation.
	# Replace sh script with sleep.
	update_config ' (.. | select(.terminal? != null)) .terminal |= false
			| (.. | select(.[]? == "sh")) += ["sleep", "1000s"]
			| del(.. | select(.? == "sh"))'

	# Make sure that the handling of non-detached IO is done properly. See #1354.
	(
		__runc run test_busybox
	) &

	wait_for_container 15 1 test_busybox running
	testcontainer test_busybox running

	runc -0 kill test_busybox KILL
}

@test "runc run -d [terminal=false]" {
	# Disable terminal creation.
	# Replace sh script with sleep.
	update_config ' (.. | select(.terminal? != null)) .terminal |= false
			| (.. | select(.[]? == "sh")) += ["sleep", "1000s"]
			| del(.. | select(.? == "sh"))'

	# Make sure that the handling of detached IO is done properly. See #1354.
	__runc run -d test_busybox

	testcontainer test_busybox running

	runc -0 kill test_busybox KILL
}
