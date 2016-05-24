#!/bin/bash

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "$BASH_SOURCE")")
RUNC="${INTEGRATION_ROOT}/../../runc"
GOPATH="${INTEGRATION_ROOT}/../../../.."

# Test data path.
TESTDATA="${INTEGRATION_ROOT}/testdata"

# Busybox image
BUSYBOX_IMAGE="$BATS_TMPDIR/busybox.tar"
BUSYBOX_BUNDLE="$BATS_TMPDIR/busyboxtest"

# hello-world in tar format
HELLO_IMAGE="$TESTDATA/hello-world.tar"
HELLO_BUNDLE="$BATS_TMPDIR/hello-world"

# CRIU PATH
CRIU="/usr/local/sbin/criu"

# Kernel version
KERNEL_VERSION="$(uname -r)"
KERNEL_MAJOR="${KERNEL_VERSION%%.*}"
KERNEL_MINOR="${KERNEL_VERSION#$KERNEL_MAJOR.}"
KERNEL_MINOR="${KERNEL_MINOR%%.*}"

# Retry a command $1 times until it succeeds. Wait $2 seconds between retries.
function retry() {
  local attempts=$1
  shift
  local delay=$1
  shift
  local i

  for ((i=0; i < attempts; i++)); do
    run "$@"
    if [[ "$status" -eq 0 ]] ; then
      return 0
    fi
    sleep $delay
  done

  echo "Command \"$@\" failed $attempts times. Output: $output"
  false
}

# retry until the given container has state
function wait_for_container() {
  local attempts=$1
  local delay=$2
  local cid=$3
  local i

  for ((i=0; i < attempts; i++)); do
    run "$RUNC" state $cid
    if [[ "$status" -eq 0 ]] ; then
      return 0
    fi
    sleep $delay
  done

  echo "runc state failed to return state $statecheck $attempts times. Output: $output"
  false
}

# retry until the given container has state
function wait_for_container_inroot() {
  local attempts=$1
  local delay=$2
  local cid=$3
  local i

  for ((i=0; i < attempts; i++)); do
    run "$RUNC" --root $4 state $cid
    if [[ "$status" -eq 0 ]] ; then
      return 0
    fi
    sleep $delay
  done

  echo "runc state failed to return state $statecheck $attempts times. Output: $output"
  false
}

function testcontainer() {
  # test state of container
  run "$RUNC" state $1
  [ "$status" -eq 0 ]
  [[ "${output}" == *"$2"* ]]
}

function setup_busybox() {
  run mkdir "$BUSYBOX_BUNDLE"
  run mkdir "$BUSYBOX_BUNDLE"/rootfs
  if [ -e "/testdata/busybox.tar" ]; then
    BUSYBOX_IMAGE="/testdata/busybox.tar"
  fi
  if [ ! -e $BUSYBOX_IMAGE ]; then
    curl -o $BUSYBOX_IMAGE -sSL 'https://github.com/jpetazzo/docker-busybox/raw/buildroot-2014.11/rootfs.tar'
  fi
  tar -C "$BUSYBOX_BUNDLE"/rootfs -xf "$BUSYBOX_IMAGE"
  cd "$BUSYBOX_BUNDLE"
  run "$RUNC" spec
}

function setup_hello() {
  run mkdir "$HELLO_BUNDLE"
  run mkdir "$HELLO_BUNDLE"/rootfs
  tar -C "$HELLO_BUNDLE"/rootfs -xf "$HELLO_IMAGE"
  cd "$HELLO_BUNDLE"
  "$RUNC" spec
  sed -i 's;"sh";"/hello";' config.json
}

function teardown_running_container() {
  run "$RUNC" list
  if [[ "${output}" == *"$1"* ]]; then
    run "$RUNC" kill $1 KILL
    retry 10 1 eval "'$RUNC' state '$1' | grep -q 'destroyed'"
    run "$RUNC" delete $1
  fi
}

function teardown_running_container_inroot() {
  run "$RUNC" --root $2 list
  if [[ "${output}" == *"$1"* ]]; then
    run "$RUNC" --root $2 kill $1 KILL
    retry 10 1 eval "'$RUNC' --root '$2' state '$1' | grep -q 'destroyed'"
    run "$RUNC" --root $2 delete $1
  fi
}

function teardown_busybox() {
  cd "$INTEGRATION_ROOT"
  teardown_running_container test_busybox
  run rm -f -r "$BUSYBOX_BUNDLE"
}

function teardown_hello() {
  cd "$INTEGRATION_ROOT"
  teardown_running_container test_hello
  run rm -f -r "$HELLO_BUNDLE"
}
