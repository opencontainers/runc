#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}


@test "kill detached busybox" {

  # start busybox detached
  run "$RUNC" start -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  # check state
  wait_for_container 15 1 test_busybox

  testcontainer test_busybox running

  run "$RUNC" kill test_busybox KILL
  [ "$status" -eq 0 ]

  retry 10 1 eval "'$RUNC' state test_busybox | grep -q 'destroyed'"

  run "$RUNC" delete test_busybox
  [ "$status" -eq 0 ]
}
