#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
}

function teardown() {
  teardown_busybox
}

@test "runc exec" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec test_busybox echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]
}

@test "runc exec --pid-file" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec --pid-file pid.txt test_busybox echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
  [[ ${lines[0]} != $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc exec --pid-file with new CWD" {
  # create pid_file directory as the CWD
  run mkdir pid_file
  [ "$status" -eq 0 ]
  run cd pid_file
  [ "$status" -eq 0 ]

  # run busybox detached
  runc run -d -b $BUSYBOX_BUNDLE --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec --pid-file pid.txt test_busybox echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
  [[ ${lines[0]} != $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc exec ls -la" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec test_busybox ls -la
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == *"total"* ]]
  [[ ${lines[1]} == *"."* ]]
  [[ ${lines[2]} == *".."* ]]
}

@test "runc exec ls -la with --cwd" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec --cwd /bin test_busybox pwd
  [ "$status" -eq 0 ]
  [[ ${output} == "/bin"* ]]
}

@test "runc exec --env" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec --env RUNC_EXEC_TEST=true test_busybox env
  [ "$status" -eq 0 ]

  [[ ${output} == *"RUNC_EXEC_TEST=true"* ]]
}

@test "runc exec --user" {
  # --user can't work in rootless containers that don't have idmap.
  [[ "$ROOTLESS" -ne 0 ]] && requires rootless_idmap

  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  runc exec --user 1000:1000 test_busybox id
  [ "$status" -eq 0 ]

  [[ "${output}" == "uid=1000 gid=1000"* ]]
}

@test "runc exec --additional-gids" {
  requires root

  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_busybox

  runc exec --user 1000:1000 --additional-gids 100 --additional-gids 99 test_busybox id 
  [ "$status" -eq 0 ]

  [[ ${output} == "uid=1000 gid=1000 groups=99(nogroup),100(users)" ]]
}

@test "runc exec --preserve-fds" {
  # run busybox detached
  runc run -d --console-socket $CONSOLE_SOCKET test_busybox
  [ "$status" -eq 0 ]

  run bash -c "cat hello > preserve-fds.test; exec 3<preserve-fds.test; $RUNC ${RUNC_USE_SYSTEMD:+--systemd-cgroup} --log /proc/self/fd/2 --root $ROOT exec --preserve-fds=1 test_busybox cat /proc/self/fd/3"
  [ "$status" -eq 0 ]

  [[ "${output}" == *"hello"* ]]
}
