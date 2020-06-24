#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
  return 0
}

@test "runc create" {
  runc create --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  testcontainer test_container created

  runc start test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running
}

@test "runc create exec" {
  runc create --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  testcontainer test_container created

  runc exec test_container true
  [ "$status" -eq 0 ]

  testcontainer test_container created

  runc start test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running
}

@test "runc create --pid-file" {
  runc create --pid-file pid.txt --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  testcontainer test_container created

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == $(__runc state test_container | jq '.pid') ]]

  # start the command
  runc start test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running
}

@test "runc create --pid-file with new CWD" {
  # create pid_file directory as the CWD
  run mkdir pid_file
  [ "$status" -eq 0 ]
  run cd pid_file
  [ "$status" -eq 0 ]

  runc create --pid-file pid.txt -b $BUNDLE --console-socket $CONSOLE_SOCKET  test_container
  [ "$status" -eq 0 ]

  testcontainer test_container created

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == $(__runc state test_container | jq '.pid') ]]

  runc start test_container
  [ "$status" -eq 0 ]

  testcontainer test_container running
}
