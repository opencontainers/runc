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
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_busybox running

  runc kill test_busybox KILL
  [ "$status" -eq 0 ]

  retry 10 1 eval "__runc state test_busybox | grep -q 'stopped'"

  # we should ensure kill work after the container stopped
  runc kill -a test_busybox 0
  [ "$status" -eq 0 ]

  runc delete test_busybox
  [ "$status" -eq 0 ]
}
