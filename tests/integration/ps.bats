#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "ps -eaf" {
  # start busybox detached
  runc run -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  # check state
  wait_for_container 15 1 test_busybox

  testcontainer test_busybox running

  runc ps test_busybox -eaf
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ UID\ +PID\ +PPID\ +C\ +STIME\ +TTY\ +TIME\ +CMD+ ]]
  [[ "${lines[1]}" == *"root"*[0-9]* ]]
}
