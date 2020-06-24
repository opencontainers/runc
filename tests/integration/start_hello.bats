#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container

  # Setup a process that terminates (instead of /bin/bash)
  update_config '.process.args = ["echo", "Hello"]' $BUNDLE
}

function teardown() {
  teardown_container
}

@test "runc run" {
  runc run test_container
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"Hello"* ]]
}

@test "runc run ({u,g}id != 0)" {
  # cannot start containers as another user in rootless setup without idmap
  [[ "$ROOTLESS" -ne 0 ]] && requires rootless_idmap

  # replace "uid": 0 with "uid": 1000
  # and do a similar thing for gid.
  update_config ' (.. | select(.uid? == 0)) .uid |= 1000
		| (.. | select(.gid? == 0)) .gid |= 100'

  runc run test_container
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"Hello"* ]]
}

@test "runc run with rootfs set to ." {
  cp config.json rootfs/.
  rm config.json
  cd rootfs
  update_config '(.. | select(. == "rootfs")) |= "."'

  runc run test_container
  [ "$status" -eq 0 ]
  [[ "${output}" == *"Hello"* ]]
}

@test "runc run --pid-file" {
  runc run --pid-file pid.txt test_container
  [ "$status" -eq 0 ]
  [[ "${output}" == *"Hello"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
}
