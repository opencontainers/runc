#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox
  run mkdir -p "$BUSYBOX_BUNDLE"/source-{accessible,inaccessible}/dir
  chmod 750 "$BUSYBOX_BUNDLE"/source-inaccessible
  run mkdir -p "$BUSYBOX_BUNDLE"/rootfs/{proc,sys,tmp}
  run mkdir -p "$BUSYBOX_BUNDLE"/rootfs/tmp/{accessible,inaccessible}
  update_config ' .process.args += ["-c", "echo HelloWorld"] '
  update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '
}

function teardown() {
  teardown_busybox
}

@test "userns without mount" {
  # run hello-world
  runc run test_userns_without_mount
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"HelloWorld"* ]]
}

@test "userns with simple mount" {
  update_config ' .mounts += [{"source": "source-accessible/dir", "destination": "/tmp/accessible", "options": ["bind"]}] '

  # run hello-world
  runc run test_userns_with_simple_mount
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"HelloWorld"* ]]
}

@test "userns with difficult mount" {
  update_config ' .mounts += [{"source": "source-inaccessible/dir", "destination": "/tmp/inaccessible", "options": ["bind"]}] '

  # run hello-world
  runc run test_userns_with_difficult_mount
  [ "$status" -eq 0 ]

  # check expected output
  [[ "${output}" == *"HelloWorld"* ]]
}
