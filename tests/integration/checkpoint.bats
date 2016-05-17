#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "checkpoint and restore" {
  if [ ! -e "$CRIU" ] ; then
    skip
  fi

  # criu does not work with external terminals so..
  # setting terminal and root:readonly: to false
  sed -i 's;"terminal": true;"terminal": false;' config.json
  sed -i 's;"readonly": true;"readonly": false;' config.json
  sed -i 's/"sh"/"sh","-c","while :; do date; sleep 1; done"/' config.json

  (
    # start busybox (not detached)
    run "$RUNC" start test_busybox
    [ "$status" -eq 0 ]
  ) &

  # check state
  wait_for_container 15 1 test_busybox

  run "$RUNC" state test_busybox
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]

  # checkpoint the running container
  run "$RUNC" --criu "$CRIU" checkpoint test_busybox
  # if you are having problems getting criu to work uncomment the following dump:
  #cat /run/opencontainer/containers/test_busybox/criu.work/dump.log
  [ "$status" -eq 0 ]

  # after checkpoint busybox is no longer running
  run "$RUNC" state test_busybox
  [ "$status" -ne 0 ]

  # restore from checkpoint
  (
    run "$RUNC" --criu "$CRIU" restore test_busybox
    [ "$status" -eq 0 ]
  ) &

  # check state
  wait_for_container 15 1 test_busybox

  # busybox should be back up and running
  run "$RUNC" state test_busybox
  [ "$status" -eq 0 ]
  [[ "${output}" == *"running"* ]]
}
