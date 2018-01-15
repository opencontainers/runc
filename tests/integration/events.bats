#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "events --stats" {
  # XXX: currently cgroups require root containers.
  requires root

  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # generate stats
  runc events --stats test_busybox
  [ "$status" -eq 0 ]
  [[ "${lines[0]}" == [\{]"\"type\""[:]"\"stats\""[,]"\"id\""[:]"\"test_busybox\""[,]* ]]
  [[ "${lines[0]}" == *"data"* ]]
}

@test "events --stats <multiple containers>" {
  # XXX: currently cgroups require root containers.
  requires root

  # run two containers
  runc run -d --console-socket $CONSOLE_SOCKET bb_one
  [ "$status" -eq 0 ]

  runc run -d --console-socket $CONSOLE_SOCKET bb_two
  [ "$status" -eq 0 ]


  # generate stats
  runc events --stats bb_one bb_two
  [ "$status" -eq 0 ]
  [ ${#lines[@]} -eq 2 ]

  grep "stats.*bb_one" <<< "${lines[@]}"
  grep "stats.*bb_two" <<< "${lines[@]}"

  teardown_running_container bb_one
  teardown_running_container bb_two
}

@test "events --interval default " {
  # XXX: currently cgroups require root containers.
  requires root

  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # spawn two sub processes (shells)
  # the first sub process is an event logger that sends stats events to events.log
  # the second sub process waits for an event that incudes test_busybox then
  # kills the test_busybox container which causes the event logger to exit
  (__runc events test_busybox > events.log) &
  (
    retry 10 1 eval "grep -q 'test_busybox' events.log"
    teardown_running_container test_busybox
  ) &
  wait # wait for the above sub shells to finish

  [ -e events.log ]

  run cat events.log
  [ "$status" -eq 0 ]
  [[ "${lines[0]}" == '{"type":"stats","id":"test_busybox"'* ]]
  [[ "${lines[0]}" == *"data"* ]]
}

@test "events --interval 1s " {
  # XXX: currently cgroups require root containers.
  requires root

  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # spawn two sub processes (shells)
  # the first sub process is an event logger that sends stats events to events.log once a second
  # the second sub process tries 3 times for an event that incudes test_busybox
  # pausing 1s between each attempt then kills the test_busybox container which
  # causes the event logger to exit
  (__runc events --interval 1s test_busybox > events.log) &
  (
    retry 3 1 eval "grep -q 'test_busybox' events.log"
    teardown_running_container test_busybox
  ) &
  wait # wait for the above sub shells to finish

  [ -e events.log ]

  run eval "grep -q 'test_busybox' events.log"
  [ "$status" -eq 0 ]
}

@test "events --interval 100ms " {
  # XXX: currently cgroups require root containers.
  requires root

  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  #prove there is no carry over of events.log from a prior test
  [ ! -e events.log ]

  # spawn two sub processes (shells)
  # the first sub process is an event logger that sends stats events to events.log once every 100ms
  # the second sub process tries 3 times for an event that incudes test_busybox
  # pausing 100s between each attempt then kills the test_busybox container which
  # causes the event logger to exit
  (__runc events --interval 100ms test_busybox > events.log) &
  (
    retry 3 0.100 eval "grep -q 'test_busybox' events.log"
    teardown_running_container test_busybox
  ) &
  wait # wait for the above sub shells to finish

  [ -e events.log ]

  run eval "grep -q 'test_busybox' events.log"
  [ "$status" -eq 0 ]
}

@test "events --interval 100ms <multiple containers>" {
  # XXX: currently cgroups require root containers.
  requires root

  # run two containers
  runc run -d --console-socket $CONSOLE_SOCKET bb_one
  [ "$status" -eq 0 ]

  runc run -d --console-socket $CONSOLE_SOCKET bb_two
  [ "$status" -eq 0 ]


  # spawn two sub processes (shells)
  # the first sub process is an event logger that sends stats events to events.log
  # the second sub process waits for an event that incudes test_busybox then
  # kills the test_busybox container which causes the event logger to exit
  (__runc events --interval 100ms bb_one bb_two >> events.log) &
  (
    retry 3 1 eval "grep -q 'bb_two' events.log"
    teardown_running_container bb_one
    teardown_running_container bb_two
  ) &
  wait # wait for the above sub shells to finish

  [ -e events.log ]

  run cat events.log
  printf '%s\n' "${lines[@]}"
  [ "$status" -eq 0 ]

  [ ${#lines[@]} -gt 2 ]

  grep "stats.*bb_one" <<< "${lines[@]}"
  grep "stats.*bb_two" <<< "${lines[@]}"

  teardown_running_container bb_one
  teardown_running_container bb_two
}

@test "events --stats non-existent-container-1 non-existent-container-2 test_busybox (bails when any container doesn't exist)" {
  # XXX: currently cgroups require root containers.
  requires root

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc events --stats non-existent-container-1 non-existent-container-2 test_busybox
  [ "$status" -ne 0 ]
  [[ "${lines[0]}" =~ "non-existent-container-1" ]]
  [[ "${lines[0]}" =~ "does not exist" ]]
  [[ "${lines[1]}" =~ "non-existent-container-2" ]]
  [[ "${lines[1]}" =~ "does not exist" ]]
  [[ "${lines[@]}" != *"test_busybox"* ]]
}

@test "events --interval non-existent-container-1 non-existent-container-2 test_busybox (bails when any container doesn't exist)" {
  # XXX: currently cgroups require root containers.
  requires root

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc events --interval 100ms non-existent-container-1 non-existent-container-2 test_busybox
  [ "$status" -ne 0 ]
  [[ "${lines[0]}" =~ "non-existent-container-1" ]]
  [[ "${lines[0]}" =~ "does not exist" ]]
  [[ "${lines[1]}" =~ "non-existent-container-2" ]]
  [[ "${lines[1]}" =~ "does not exist" ]]
  [[ "${lines[@]}" != *"test_busybox"* ]]
}

@test "events --interval test_busybox_first test_busybox_second (should continue when any container dies)" {
  # XXX: currently cgroups require root containers.
  requires root

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox_first
  [ "$status" -eq 0 ]

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox_second
  [ "$status" -eq 0 ]

  (__runc events --interval 100ms test_busybox_first test_busybox_second > events.log && echo $? > runc_exitcode) &

  retry 3 1 eval "grep -q test_busybox_second events.log"
  teardown_running_container test_busybox_second

  : > events.log
  retry 3 1 eval "grep -q test_busybox_first events.log"
  teardown_running_container test_busybox_first

  wait

  run cat events.log
  printf '%s\n' "${lines[@]}"

  [[ "${lines[@]}" != *"test_busybox_second"* ]]
  [ "$(cat runc_exitcode)" == "0" ]
}

@test "events --interval test_busybox (should report oom events)" {
  # XXX: currently cgroups require root containers.
  requires root

  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  # set memory limit to 10M
  runc update test_busybox --memory "10M"
  [ "$status" -eq 0 ]

  (__runc events --interval 100ms test_busybox > events.log ) &

  retry 3 1 eval "grep -q 'test_busybox' events.log"

  # cause OOM
  runc exec test_busybox dd if=/dev/urandom of=/dev/shm/too-big bs=1M count=12
  [ "$status" -ne 0 ]

  teardown_running_container test_busybox
  wait

  [ -e events.log ]

  run cat events.log
  printf '%s\n' "${lines[@]}"
  grep "oom.*test_busybox" <<< "${lines[@]}"
}
