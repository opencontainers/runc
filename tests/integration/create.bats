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
  runc create test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox created

  # start the command
  runc start test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running
}

@test "runc create exec" {
  runc create test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox created

  runc exec test_busybox true
  [ "$status" -eq 0 ]

  # start the command
  runc start test_busybox
  [ "$status" -eq 0 ]

  testcontainer test_busybox running
}
