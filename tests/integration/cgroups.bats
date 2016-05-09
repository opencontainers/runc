#!/usr/bin/env bats

load helpers

UPDATE_TEST_RUNC_ROOT="$BATS_TMPDIR/runc-cgroups-integration-test"

CGROUP_MEMORY=""

TEST_CGROUP_NAME="runc-cgroups-integration-test"

function init_cgroup_path() {
   base_path=$(grep "rw,"  /proc/self/mountinfo | grep -i -m 1 'MEMORY$' | cut -d ' ' -f 5)
   CGROUP_MEMORY="${base_path}/${TEST_CGROUP_NAME}"
}

function teardown() {
    rm -f $BATS_TMPDIR/runc-update-integration-test.json
    teardown_running_container_inroot test_cgroups_kmem $UPDATE_TEST_RUNC_ROOT
    teardown_busybox
}

function setup() {
    teardown
    setup_busybox

    init_cgroup_path
}

function check_cgroup_value() {
    cgroup=$1
    source=$2
    expected=$3

    current=$(cat $cgroup/$source)
    echo  $cgroup/$source
    echo "current" $current "!?" "$expected"
    [ "$current" -eq "$expected" ]
}

@test "cgroups-kernel-memory-initialized" {
    # Add cgroup path
    sed -i 's/\("linux": {\)/\1\n    "cgroupsPath": "runc-cgroups-integration-test",/'  ${BUSYBOX_BUNDLE}/config.json

    # Set some initial known values
    DATA=$(cat <<-EOF
    "memory": {
        "kernel": 16777216
    },
EOF
    )
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # start a detached busybox to work with
    "$RUNC" --root $UPDATE_TEST_RUNC_ROOT start -d --console /dev/pts/ptmx test_cgroups_kmem
    [ "$status" -eq 0 ]
    wait_for_container_inroot 15 1 test_cgroups_kmem $UPDATE_TEST_RUNC_ROOT

    # update kernel memory limit
    "$RUNC" --root $UPDATE_TEST_RUNC_ROOT update test_cgroups_kmem --kernel-memory 50331648
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 50331648
}

@test "cgroups-kernel-memory-uninitialized" {
    # Add cgroup path
    sed -i 's/\("linux": {\)/\1\n    "cgroupsPath": "runc-cgroups-integration-test",/'  ${BUSYBOX_BUNDLE}/config.json

    # start a detached busybox to work with
    run "$RUNC" --root $UPDATE_TEST_RUNC_ROOT start -d --console /dev/pts/ptmx test_cgroups_kmem
    [ "$status" -eq 0 ]
    wait_for_container_inroot 15 1 test_cgroups_kmem $UPDATE_TEST_RUNC_ROOT

    # update kernel memory limit
    run "$RUNC" --root $UPDATE_TEST_RUNC_ROOT update test_cgroups_kmem --kernel-memory 50331648
    [ ! "$status" -eq 0 ]
}
