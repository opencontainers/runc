#!/usr/bin/env bats

load helpers

function setup() {
  teardown_running_container_inroot test_dotbox $BUNDLE
  teardown_container
  setup_container
}

function teardown() {
  teardown_running_container_inroot test_dotbox $BUNDLE
  teardown_container
}

@test "global --root" {
  # run container detached using $BUNDLE for state
  ROOT=$BUNDLE runc run -d --console-socket $CONSOLE_SOCKET test_dotbox
  [ "$status" -eq 0 ]

  # run container detached in default root
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc state test_container
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]

  ROOT=$BUNDLE runc state test_dotbox
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]

  ROOT=$BUNDLE runc state test_container
  [ "$status" -ne 0 ]

  runc state test_dotbox
  [ "$status" -ne 0 ]

  runc kill test_container KILL
  [ "$status" -eq 0 ]
  retry 10 1 eval "__runc state test_container | grep -q 'stopped'"
  runc delete test_container
  [ "$status" -eq 0 ]

  ROOT=$BUNDLE runc kill test_dotbox KILL
  [ "$status" -eq 0 ]
  retry 10 1 eval "ROOT='$BUNDLE' __runc state test_dotbox | grep -q 'stopped'"
  ROOT=$BUNDLE runc delete test_dotbox
  [ "$status" -eq 0 ]
}
