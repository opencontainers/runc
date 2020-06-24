#!/usr/bin/env bats

load helpers

function setup() {
  teardown_container
  setup_container
}

function teardown() {
  teardown_container
}

@test "runc delete" {
  runc run -d --console-socket $CONSOLE_SOCKET testcontainerdelete
  [ "$status" -eq 0 ]

  testcontainer testcontainerdelete running

  runc kill testcontainerdelete KILL
  [ "$status" -eq 0 ]
  retry 10 1 eval "__runc state testcontainerdelete | grep -q 'stopped'"

  runc delete testcontainerdelete
  [ "$status" -eq 0 ]

  runc state testcontainerdelete
  [ "$status" -ne 0 ]

  run find /sys/fs/cgroup -wholename '*testcontainerdelete*' -type d
  [ "$status" -eq 0 ]
  [ "$output" = "" ] || fail "cgroup not cleaned up correctly: $output"
}

@test "runc delete --force" {
  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  runc delete --force test_container

  runc state test_container
  [ "$status" -ne 0 ]
}

@test "runc delete --force ignore not exist" {
  runc delete --force notexists
  [ "$status" -eq 0 ]
}

@test "runc delete --force in cgroupv2 with subcgroups" {
  requires cgroups_v2 root
  set_cgroups_path "$BUNDLE"
  set_cgroup_mount_writable "$BUNDLE"

  runc run -d --console-socket $CONSOLE_SOCKET test_container
  [ "$status" -eq 0 ]

  # check state
  testcontainer test_container running

  # create a sub process
  __runc exec -d test_container sleep 1d

  # find the pid of sleep
  pid=$(__runc exec test_container ps ax | grep 1d | awk '{print $1}')
  [[ ${pid} =~ [0-9]+ ]]

  # create subcgroups
  cat <<EOF > nest.sh
  set -e -u -x
  cd /sys/fs/cgroup
  echo +pids > cgroup.subtree_control
  mkdir foo
  cd foo
  echo threaded > cgroup.type
  echo ${pid} > cgroup.threads
  cat cgroup.threads
EOF
  cat nest.sh | runc exec test_container sh
  [ "$status" -eq 0 ]
  [[ "$output" =~ [0-9]+ ]]

  # check create subcgroups success
  [ -d $CGROUP_PATH/foo ]

  runc delete --force test_container

  runc state test_container
  [ "$status" -ne 0 ]

  # check delete subcgroups success
  [ ! -d $CGROUP_PATH/foo ]
}
