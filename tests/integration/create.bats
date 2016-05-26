#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "runc create" {
  run "$RUNC" create --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox created

  # start the command
  run "$RUNC" start test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running
}

@test "runc create exec" {
  run "$RUNC" create --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox created

  run "$RUNC" exec test_busybox true
  [ "$status" -eq 0 ]

  # start the command
  run "$RUNC" start test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running
}
