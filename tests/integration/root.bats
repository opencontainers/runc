#!/usr/bin/env bats

load helpers

function setup() {
  teardown_running_container_inroot test_dotbox $HELLO_BUNDLE
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_running_container_inroot test_dotbox $HELLO_BUNDLE
  teardown_busybox
}

@test "global --root" {
  # start busybox detached using $HELLO_BUNDLE for state
  run "$RUNC" --root $HELLO_BUNDLE start -d --console /dev/pts/ptmx test_dotbox
  [ "$status" -eq 0 ]

  # start busybox detached in default root
  run "$RUNC" start -d --console /dev/pts/ptmx test_busybox
  [ "$status" -eq 0 ]

  # check state of the busyboxes are only in their respective root path
  wait_for_container 15 1 test_busybox
  wait_for_container_inroot 15 1 test_dotbox $HELLO_BUNDLE

  run "$RUNC" state test_busybox
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]

  run "$RUNC" --root $HELLO_BUNDLE state test_dotbox
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]

  run "$RUNC" --root $HELLO_BUNDLE state test_busybox
  [ "$status" -ne 0 ]

  run "$RUNC" state test_dotbox
  [ "$status" -ne 0 ]

  run "$RUNC" kill test_busybox KILL
  [ "$status" -eq 0 ]
  retry 10 1 eval "'$RUNC' state test_busybox | grep -q 'destroyed'"
  run "$RUNC" delete test_busybox
  [ "$status" -eq 0 ]

  run "$RUNC" --root $HELLO_BUNDLE kill test_dotbox KILL
  [ "$status" -eq 0 ]
  retry 10 1 eval "'$RUNC' --root $HELLO_BUNDLE state test_dotbox | grep -q 'destroyed'"
  run "$RUNC" --root $HELLO_BUNDLE delete test_dotbox
  [ "$status" -eq 0 ]
}
