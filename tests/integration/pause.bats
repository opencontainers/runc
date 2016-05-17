#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "runc pause and resume" {
  # start busybox detached
  run "$RUNC" start -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  # pause busybox
  run "$RUNC" pause test_busybox
  [ "$status" -eq 0 ]

  # test state of busybox is paused
  testcontainer test_busybox paused

  # resume busybox
  run "$RUNC" resume test_busybox
  [ "$status" -eq 0 ]

  # test state of busybox is back to running
  testcontainer test_busybox running
}
