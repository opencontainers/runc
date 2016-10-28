#!/usr/bin/env bats

load helpers

function setup() {
  teardown_busybox
  setup_busybox

  # Create a "shared" parent mount point for container
  # rootfs and make sure pivot_root() still works. This
  cd ${BUSYBOX_BUNDLE}
  mount --bind . .
  mount --make-private .
  mount --make-shared .

  # Also use rootPropagation=private (and not rprivate) so that shared volumes
  # can possibly work.
  sed -i '/"linux": {/ a "rootfsPropagation":"private"', config.json
}

function teardown() {
  cd ${BUSYBOX_BUNDLE}/../
  umount -R ${BUSYBOX_BUNDLE}
  teardown_busybox
}

@test "mount propagation parent mount shared" {
  runc create --console /dev/pts/ptmx test_busybox1
  [ "$status" -eq 0 ]

  testcontainer test_busybox1 created

  # start conatiner test_busybox1
  runc start test_busybox1
  [ "$status" -eq 0 ]

  testcontainer test_busybox1 running

  # delete test_busybox1
  runc delete --force test_busybox1

  runc state test_busybox1
  [ "$status" -ne 0 ]
}
