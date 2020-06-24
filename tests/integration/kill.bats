#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}


@test "kill detached container" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  runc kill test_container KILL
  [ "$status" -eq 0 ]

  retry 10 1 eval "__runc state test_container | grep -q 'stopped'"

  # we should ensure kill work after the container stopped
  runc kill -a test_container 0
  [ "$status" -eq 0 ]

  runc delete test_container
  [ "$status" -eq 0 ]
}
