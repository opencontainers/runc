#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container

  # Setup a process that terminates (instead of /bin/bash)
  update_config '.process.args = ["echo", "DEFAULT_COMMAND"]' $BUNDLE
}

function teardown() {
  teardown_container
}

@test "global --debug" {
  runc --debug run test_container
  echo "${output}"
  [ "$status" -eq 0 ]

  # check expected debug output was sent to stderr
  [[ "${output}" == *"level=debug"* ]]
  [[ "${output}" == *"nsexec started"* ]]
  [[ "${output}" == *"child process in init()"* ]]
}

@test "global --debug to --log" {
  runc --log log.out --debug run test_container
  [ "$status" -eq 0 ]

  # check output does not include debug info
  [[ "${output}" != *"level=debug"* ]]

  # check log.out was generated
  [ -e log.out ]

  # check expected debug output was sent to log.out
  run cat log.out
  [ "$status" -eq 0 ]
  [[ "${output}" == *"level=debug"* ]]
  [[ "${output}" == *"nsexec started"* ]]
  [[ "${output}" == *"child process in init()"* ]]
}

@test "global --debug to --log --log-format 'text'" {
  runc --log log.out --log-format "text" --debug run test_container
  [ "$status" -eq 0 ]

  # check output does not include debug info
  [[ "${output}" != *"level=debug"* ]]

  # check log.out was generated
  [ -e log.out ]

  # check expected debug output was sent to log.out
  run cat log.out
  [ "$status" -eq 0 ]
  [[ "${output}" == *"level=debug"* ]]
  [[ "${output}" == *"nsexec started"* ]]
  [[ "${output}" == *"child process in init()"* ]]
}

@test "global --debug to --log --log-format 'json'" {
  runc --log log.out --log-format "json" --debug run test_container
  [ "$status" -eq 0 ]

  # check output does not include debug info
  [[ "${output}" != *"level=debug"* ]]

  # check log.out was generated
  [ -e log.out ]

  # check expected debug output was sent to log.out
  run cat log.out
  [ "$status" -eq 0 ]
  [[ "${output}" == *'"level":"debug"'* ]]
  [[ "${output}" == *"nsexec started"* ]]
  [[ "${output}" == *"child process in init()"* ]]
}
