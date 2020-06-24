#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}

@test "runc pause and resume" {
  if [[ "$ROOTLESS" -ne 0 ]]
  then
    requires rootless_cgroup
    set_cgroups_path "$BUNDLE"
  fi
  requires cgroups_freezer

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running

  # pause container
  runc pause test_container
  [ "$status" -eq 0 ]

  # test state of container is paused
  testcontainer test_container paused

  # resume container
  runc resume test_container
  [ "$status" -eq 0 ]

  # test state of container is back to running
  testcontainer test_container running
}

@test "runc pause and resume with nonexist container" {
  if [[ "$ROOTLESS" -ne 0 ]]
  then
    requires rootless_cgroup
    set_cgroups_path "$BUNDLE"
  fi
  requires cgroups_freezer

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running

  # pause test_container and nonexistent container
  runc pause test_container
  [ "$status" -eq 0 ]
  runc pause nonexistent
  [ "$status" -ne 0 ]

  # test state of test_container is paused
  testcontainer test_container paused

  # resume test_container and nonexistent container
  runc resume test_container
  [ "$status" -eq 0 ]
  runc resume nonexistent
  [ "$status" -ne 0 ]

  # test state of test_container is back to running
  testcontainer test_container running

  runc delete --force test_container

  runc state test_container
  [ "$status" -ne 0 ]
}
