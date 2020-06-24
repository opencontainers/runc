#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}

@test "runc run detached" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running
}

@test "runc run detached ({u,g}id != 0)" {
  # cannot start containers as another user in rootless setup without idmap
  [[ "$ROOTLESS" -ne 0 ]] && requires rootless_idmap

  # replace "uid": 0 with "uid": 1000
  # and do a similar thing for gid.
  update_config ' (.. | select(.uid? == 0)) .uid |= 1000
		| (.. | select(.gid? == 0)) .gid |= 100' 

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running
}

@test "runc run detached --pid-file" {
  runc run --pid-file pid.txt -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == $(__runc state test_container | jq '.pid') ]]
}

@test "runc run detached --pid-file with new CWD" {
  # create pid_file directory as the CWD
  run mkdir pid_file
  [ "$status" -eq 0 ]
  run cd pid_file
  [ "$status" -eq 0 ]

  runc run --pid-file pid.txt -d  -b $BUNDLE --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == $(__runc state test_container | jq '.pid') ]]
}
