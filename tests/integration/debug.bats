#!/usr/bin/env bats

load helpers

function setup() {
  teardown_hello
  setup_hello
}

function teardown() {
  teardown_hello
}

@test "global --debug" {
  # start hello-world
  runc --debug start test_hello
  echo "${output}"
  [ "$status" -eq 0 ]
}

@test "global --debug to --log" {
  # start hello-world
  runc --log log.out --debug start test_hello
  [ "$status" -eq 0 ]

  # check output does not include debug info
  [[ "${output}" != *"level=debug"* ]]

  # check log.out was generated
  [ -e log.out ]

  # check expected debug output was sent to log.out
  run cat log.out
  [ "$status" -eq 0 ]
  [[ "${output}" == *"level=debug"* ]]
}

@test "global --debug to --log --log-format 'text'" {
  # start hello-world
  runc --log log.out --log-format "text" --debug start test_hello
  [ "$status" -eq 0 ]

  # check output does not include debug info
  [[ "${output}" != *"level=debug"* ]]

  # check log.out was generated
  [ -e log.out ]

  # check expected debug output was sent to log.out
  run cat log.out
  [ "$status" -eq 0 ]
  [[ "${output}" == *"level=debug"* ]]
}

@test "global --debug to --log --log-format 'json'" {
  # start hello-world
  runc --log log.out --log-format "json" --debug start test_hello
  [ "$status" -eq 0 ]

  # check output does not include debug info
  [[ "${output}" != *"level=debug"* ]]

  # check log.out was generated
  [ -e log.out ]

  # check expected debug output was sent to log.out
  run cat log.out
  [ "$status" -eq 0 ]
  [[ "${output}" == *'"level":"debug"'* ]]
}
