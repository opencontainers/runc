#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "runc start detached" {
  # start busybox detached
  runc start -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  # check state
  wait_for_container 15 1 test_busybox

  testcontainer test_busybox running
}

@test "runc start detached --pid-file" {
  # start busybox detached
  runc start --pid-file pid.txt -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  # check state
  wait_for_container 15 1 test_busybox

  testcontainer test_busybox running

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
}
