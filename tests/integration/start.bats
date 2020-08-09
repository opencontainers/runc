#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}

@test "runc start" {
  runc create --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  testcontainer test_container created

  # start container test_container
  runc start test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running

  # delete test_container
  runc delete --force test_container

  runc state test_container
  [ "$status" -ne 0 ]
}
