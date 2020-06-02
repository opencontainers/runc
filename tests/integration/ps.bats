#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "ps" {
  # ps is not supported, it requires cgroups
  requires root

  # start busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_busybox running

  runc ps test_busybox
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ UID\ +PID\ +PPID\ +C\ +STIME\ +TTY\ +TIME\ +CMD+ ]]
  [[ "${lines[1]}" == *"$(id -un 2>/dev/null)"*[0-9]* ]]
}

@test "ps -f json" {
  # ps is not supported, it requires cgroups
  requires root

  # start busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_busybox running

  runc ps -f json test_busybox
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
}

@test "ps -e -x" {
  # ps is not supported, it requires cgroups
  requires root

  # start busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_busybox running

  runc ps test_busybox -e -x
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ \ +PID\ +TTY\ +STAT\ +TIME\ +COMMAND+ ]]
  [[ "${lines[1]}" =~ [0-9]+ ]]
}

@test "ps after the container stopped" {
  # ps requires cgroups
  [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
  set_cgroups_path "$BUSYBOX_BUNDLE"

  # start busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_busybox running

  runc ps test_busybox
  [ "$status" -eq 0 ]

  runc kill test_busybox KILL
  [ "$status" -eq 0 ]

  retry 10 1 eval "__runc state test_busybox | grep -q 'stopped'"

  runc ps test_busybox
  [ "$status" -eq 0 ]
}
