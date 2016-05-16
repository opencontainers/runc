#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "state" {
  run "$RUNC" state test_busybox
  [ "$status" -ne 0 ]

  # start busybox detached
  run "$RUNC" start -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  # check state
  wait_for_container 15 1 test_busybox

  testcontainer test_busybox running

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

  run "$RUNC" kill test_busybox KILL
  # wait for busybox to be in the destroyed state
  retry 10 1 eval "'$RUNC' state test_busybox | grep -q 'destroyed'"

  # delete test_busybox
  run "$RUNC" delete test_busybox

  run "$RUNC" state test_busybox
  [ "$status" -ne 0 ]
}
