#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	ALT_ROOT="$ROOT/alt"
	mkdir -p "$ALT_ROOT/state"
}

function teardown() {
	ROOT="$ALT_ROOT" teardown_bundle
	unset ALT_ROOT
	teardown_bundle
}

@test "list" {
	bundle=$(pwd)
	ROOT=$ALT_ROOT runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_box1

	ROOT=$ALT_ROOT runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_box2

	ROOT=$ALT_ROOT runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_box3

	ROOT=$ALT_ROOT runc -0 list
	[[ ${lines[0]} =~ ID\ +PID\ +STATUS\ +BUNDLE\ +CREATED+ ]]
	[[ "${lines[1]}" == *"test_box1"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[2]}" == *"test_box2"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[3]}" == *"test_box3"*[0-9]*"running"*$bundle*[0-9]* ]]

	ROOT=$ALT_ROOT runc -0 list -q
	[ "${lines[0]}" = "test_box1" ]
	[ "${lines[1]}" = "test_box2" ]
	[ "${lines[2]}" = "test_box3" ]

	ROOT=$ALT_ROOT runc -0 list --format table
	[[ ${lines[0]} =~ ID\ +PID\ +STATUS\ +BUNDLE\ +CREATED+ ]]
	[[ "${lines[1]}" == *"test_box1"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[2]}" == *"test_box2"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[3]}" == *"test_box3"*[0-9]*"running"*$bundle*[0-9]* ]]

	ROOT=$ALT_ROOT runc -0 list --format json
	[[ "${lines[0]}" == [\[][\{]"\"ociVersion\""[:]"\""*[0-9][\.]*[0-9][\.]*[0-9]*"\""[,]"\"id\""[:]"\"test_box1\""[,]"\"pid\""[:]*[0-9][,]"\"status\""[:]*"\"running\""[,]"\"bundle\""[:]*$bundle*[,]"\"rootfs\""[:]"\""*"\""[,]"\"created\""[:]*[0-9]*[\}]* ]]
	[[ "${lines[0]}" == *[,][\{]"\"ociVersion\""[:]"\""*[0-9][\.]*[0-9][\.]*[0-9]*"\""[,]"\"id\""[:]"\"test_box2\""[,]"\"pid\""[:]*[0-9][,]"\"status\""[:]*"\"running\""[,]"\"bundle\""[:]*$bundle*[,]"\"rootfs\""[:]"\""*"\""[,]"\"created\""[:]*[0-9]*[\}]* ]]
	[[ "${lines[0]}" == *[,][\{]"\"ociVersion\""[:]"\""*[0-9][\.]*[0-9][\.]*[0-9]*"\""[,]"\"id\""[:]"\"test_box3\""[,]"\"pid\""[:]*[0-9][,]"\"status\""[:]*"\"running\""[,]"\"bundle\""[:]*$bundle*[,]"\"rootfs\""[:]"\""*"\""[,]"\"created\""[:]*[0-9]*[\}][\]] ]]
}
