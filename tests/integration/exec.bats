#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}

@test "runc exec" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec test_container echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]
}

@test "runc exec --pid-file" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec --pid-file pid.txt test_container echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
  [[ ${lines[0]} != $(__runc state test_container | jq '.pid') ]]
}

@test "runc exec --pid-file with new CWD" {
  # create pid_file directory as the CWD
  run mkdir pid_file
  [ "$status" -eq 0 ]
  run cd pid_file
  [ "$status" -eq 0 ]

  runc run -d -b $BUNDLE --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec --pid-file pid.txt test_container echo Hello from exec
  [ "$status" -eq 0 ]
  echo text echoed = "'""${output}""'"
  [[ "${output}" == *"Hello from exec"* ]]

  # check pid.txt was generated
  [ -e pid.txt ]

  run cat pid.txt
  [ "$status" -eq 0 ]
  [[ ${lines[0]} =~ [0-9]+ ]]
  [[ ${lines[0]} != $(__runc state test_container | jq '.pid') ]]
}

@test "runc exec ls -la" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec test_container ls -la
  [ "$status" -eq 0 ]
  [[ ${lines[0]} == *"total"* ]]
  [[ ${lines[1]} == *"."* ]]
  [[ ${lines[2]} == *".."* ]]
}

@test "runc exec ls -la with --cwd" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec --cwd /bin test_container pwd
  [ "$status" -eq 0 ]
  [[ ${output} == "/usr/bin"* ]]
}

@test "runc exec --env" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec --env RUNC_EXEC_TEST=true test_container env
  [ "$status" -eq 0 ]

  [[ ${output} == *"RUNC_EXEC_TEST=true"* ]]
}

@test "runc exec --user" {
  # --user can't work in rootless containers that don't have idmap.
  [[ "$ROOTLESS" -ne 0 ]] && requires rootless_idmap

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  runc exec --user 1000:1000 test_container id
  [ "$status" -eq 0 ]

  [[ "${output}" == "uid=1000 gid=1000"* ]]
}

@test "runc exec --additional-gids" {
  requires root

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  wait_for_container 15 1 test_container

  runc exec --user 1000:1000 --additional-gids 100 --additional-gids 65534 test_container id
  [ "$status" -eq 0 ]

  echo "${output}"
  [[ ${output} == "uid=1000 gid=1000 groups=1000,100(users),65534(nogroup)" ]]
}

@test "runc exec --preserve-fds" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  run bash -c "cat container > preserve-fds.test; exec 3<preserve-fds.test; $RUNC ${RUNC_USE_SYSTEMD:+--systemd-cgroup} --log /proc/self/fd/2 --root $ROOT exec --preserve-fds=1 test_container cat /proc/self/fd/3"
  [ "$status" -eq 0 ]

  [[ "${output}" == *"container"* ]]
}
