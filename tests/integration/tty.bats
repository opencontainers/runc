#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

@test "runc create [terminal=false]" {
	# Replace sh script with sleep -- because sh will exit if stdio is /dev/null.
	sed -i 's|"sh"|"sleep", "9999999"|' config.json
	sed -i 's|"terminal": true|"terminal": false|' config.json

	runc create test_busybox
	[ "$status" -eq 0 ]

	# check state
	wait_for_container 15 1 test_busybox

	# make sure we're created
	testcontainer test_busybox created

	runc exec test_busybox true
	[ "$status" -eq 0 ]

	runc start test_busybox
	[ "$status" -eq 0 ]

	# check state
	wait_for_container 15 1 test_busybox

	# make sure we're running
	testcontainer test_busybox running

	runc exec test_busybox sh -c 'for file in /proc/1/fd/[012]; do readlink $file; done'
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "/dev/null" ]]
	[[ "${lines[1]}" == "/dev/null" ]]
	[[ "${lines[2]}" == "/dev/null" ]]
}

@test "runc run -d [terminal=false]" {
	# Replace sh script with sleep -- because sh will exit if stdio is /dev/null.
	sed -i 's|"sh"|"sleep", "9999999"|' config.json
	sed -i 's|"terminal": true|"terminal": false|' config.json

	runc run -d test_busybox
	[ "$status" -eq 0 ]

	# check state
	wait_for_container 15 1 test_busybox

	# make sure we're running
	testcontainer test_busybox running

	runc exec test_busybox sh -c 'for file in /proc/1/fd/[012]; do readlink $file; done'
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "/dev/null" ]]
	[[ "${lines[1]}" == "/dev/null" ]]
	[[ "${lines[2]}" == "/dev/null" ]]
}

@test "runc run [tty ptsname]" {
	# Replace sh script with readlink.
    sed -i 's|"sh"|"sh", "-c", "for file in /proc/self/fd/[012]; do readlink $file; done"|' config.json

	# run busybox
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ /dev/pts/+ ]]
	[[ ${lines[1]} =~ /dev/pts/+ ]]
	[[ ${lines[2]} =~ /dev/pts/+ ]]
}

@test "runc run [tty owner]" {
	# Replace sh script with stat.
	sed -i 's/"sh"/"sh", "-c", "stat -c %u:%g $(tty) | tr : \\\\\\\\n"/' config.json

	# run busybox
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ 0 ]]
	# This is set by the default config.json (it corresponds to the standard tty group).
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc run [tty owner] ({u,g}id != 0)" {
	# replace "uid": 0 with "uid": 1000
	# and do a similar thing for gid.
	sed -i 's;"uid": 0;"uid": 1000;g' config.json
	sed -i 's;"gid": 0;"gid": 100;g' config.json

	# Replace sh script with stat.
	sed -i 's/"sh"/"sh", "-c", "stat -c %u:%g $(tty) | tr : \\\\\\\\n"/' config.json

	# run busybox
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ 1000 ]]
	# This is set by the default config.json (it corresponds to the standard tty group).
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc exec [tty ptsname]" {
	# run busybox detached
	runc run -d --console-socket $CONSOLE_SOCKET test_busybox
	[ "$status" -eq 0 ]

	# check state
	wait_for_container 15 1 test_busybox

	# make sure we're running
	testcontainer test_busybox running

	# run the exec
    runc exec test_busybox sh -c 'for file in /proc/self/fd/[012]; do readlink $file; done'
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ /dev/pts/+ ]]
	[[ ${lines[1]} =~ /dev/pts/+ ]]
	[[ ${lines[2]} =~ /dev/pts/+ ]]
}

@test "runc exec [tty owner]" {
	# run busybox detached
	runc run -d --console-socket $CONSOLE_SOCKET test_busybox
	[ "$status" -eq 0 ]

	# check state
	wait_for_container 15 1 test_busybox

	# make sure we're running
	testcontainer test_busybox running

	# run the exec
    runc exec test_busybox sh -c 'stat -c %u:%g $(tty) | tr : \\n'
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ 0 ]]
	[[ ${lines[1]} =~ 5 ]]
}

@test "runc exec [tty owner] ({u,g}id != 0)" {
	# replace "uid": 0 with "uid": 1000
	# and do a similar thing for gid.
	sed -i 's;"uid": 0;"uid": 1000;g' config.json
	sed -i 's;"gid": 0;"gid": 100;g' config.json

	# run busybox detached
	runc run -d --console-socket $CONSOLE_SOCKET test_busybox
	[ "$status" -eq 0 ]

	# check state
	wait_for_container 15 1 test_busybox

	# make sure we're running
	testcontainer test_busybox running

	# run the exec
    runc exec test_busybox sh -c 'stat -c %u:%g $(tty) | tr : \\n'
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ 1000 ]]
	[[ ${lines[1]} =~ 5 ]]
}
