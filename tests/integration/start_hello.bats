#!/usr/bin/env bats

load helpers

function setup() {
  teardown_hello
  setup_hello
}

function teardown() {
  teardown_hello
}

@test "runc start" {
  # start hello-world
  runc start test_hello
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"Hello"* ]]
}

@test "runc start with rootfs set to ." {
  cp config.json rootfs/.
  rm config.json
  cd rootfs
  sed -i 's;"rootfs";".";' config.json

  # start hello-world
  runc start test_hello
  [ "$status" -eq 0 ]
  [[ "${output}" == *"Hello"* ]]
}

@test "runc start --pid-file" {
  # start hello-world
  runc start --pid-file pid.txt test_hello
  [ "$status" -eq 0 ]
  [[ "${output}" == *"Hello"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
}
