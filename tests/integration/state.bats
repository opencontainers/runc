#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}

@test "state (kill + delete)" {
  runc state test_container
  [ "$status" -ne 0 ]

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  runc kill test_container KILL
  [ "$status" -eq 0 ]

  # wait for container to be in the destroyed state
  retry 10 1 eval "__runc state test_container | grep -q 'stopped'"

  runc delete test_container
  [ "$status" -eq 0 ]

  runc state test_container
  [ "$status" -ne 0 ]
}

@test "state (pause + resume)" {
  # XXX: pause and resume require cgroups.
  requires root

  runc state test_container
  [ "$status" -ne 0 ]

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  runc pause test_container
  [ "$status" -eq 0 ]

  # test state of container is paused
  testcontainer test_container paused

  runc resume test_container
  [ "$status" -eq 0 ]

  # test state of container is back to running
  testcontainer test_container running
}
