#!/usr/bin/env bats

load helpers

function setup() {
  teardown_hello
  setup_hello
}

function teardown() {
  teardown_hello
}

@test "runc run" {
  # run hello-world
  runc run test_hello
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"Hello"* ]]
}

@test "runc run with rootfs set to ." {
  cp config.json rootfs/.
  rm config.json
  cd rootfs
  sed -i 's;"rootfs";".";' config.json

  # run hello-world
  runc run test_hello
  [ "$status" -eq 0 ]
  [[ "${output}" == *"Hello"* ]]
}

@test "runc run --pid-file" {
  # run hello-world
  runc run --pid-file pid.txt test_hello
  [ "$status" -eq 0 ]
  [[ "${output}" == *"Hello"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
}
