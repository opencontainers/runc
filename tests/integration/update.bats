#!/usr/bin/env bats

load helpers

function teardown() {
    rm -f $BATS_TMPDIR/runc-update-integration-test.json
    teardown_running_container test_update
    teardown_running_container test_update_rt
    teardown_busybox
}

function setup() {
    teardown
    setup_busybox

    # Add cgroup path
    sed -i 's/\("linux": {\)/\1\n    "cgroupsPath": "\/runc-update-integration-test",/'  ${BUSYBOX_BUNDLE}/config.json
}

function check_cgroup_value() {
    cgroup=$1
    source=$2
    expected=$3

    current=$(cat $cgroup/$source)
    [ "$current" == "$expected" ]
}

@test "runc update memory" {
    requires root

    DATA=$(cat <<EOF
    "memory": {
        "limit": 33554432,
        "reservation": 25165824
    },
EOF
    )
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    CGROUP_MEMORY_ROOT=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'MEMORY'\>/ { print $5; exit }')
    CGROUP_MEMORY="${CGROUP_MEMORY_ROOT}/runc-update-integration-test"

    # check that initial values were properly set
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 33554432
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 25165824

    # update memory limit
    runc update test_update --memory 67108864
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 67108864

    runc update test_update --memory 50M
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 52428800

    # update memory soft limit
    runc update test_update --memory-reservation 33554432
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 33554432

    # Get unlimited memory value
    UNLIMITED_MEMORY=$(cat "${CGROUP_MEMORY_ROOT}/memory.limit_in_bytes")

    # Run swap memory tests if swap is available
    if [ -f "$CGROUP_MEMORY/memory.memsw.limit_in_bytes" ]; then
        # try to remove memory swap limit
        runc update test_update --memory-swap -1
        [ "$status" -eq 0 ]
        check_cgroup_value $CGROUP_MEMORY "memory.memsw.limit_in_bytes" ${UNLIMITED_MEMORY}

        # update memory swap
        runc update test_update --memory-swap 96468992
        [ "$status" -eq 0 ]
        check_cgroup_value $CGROUP_MEMORY "memory.memsw.limit_in_bytes" 96468992
    fi;

    # try to remove memory limit
    runc update test_update --memory -1
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" ${UNLIMITED_MEMORY}

    # check if swap memory limit is gone
    if [ -f "$CGROUP_MEMORY/memory.memsw.limit_in_bytes" ]; then
        check_cgroup_value $CGROUP_MEMORY "memory.memsw.limit_in_bytes" ${UNLIMITED_MEMORY}
    fi

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "memory": {
    "limit": 33554432,
    "reservation": 25165824
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 33554432
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 25165824

    # redo all the changes at once
    runc update test_update --memory 67108864 --memory-reservation 33554432
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 67108864
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 33554432

    # reset to initial test value via json file
    DATA=$(cat <<"EOF"
{
  "memory": {
    "limit": 33554432,
    "reservation": 25165824
  }
}
EOF
)
    echo $DATA > $BATS_TMPDIR/runc-update-integration-test.json
    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.limit_in_bytes" 33554432
    check_cgroup_value $CGROUP_MEMORY "memory.soft_limit_in_bytes" 25165824
}

@test "runc update kernel memory" {
    requires cgroups_kmem root

    DATA=$(cat <<EOF
    "memory": {
        "kernel": 16777216,
        "kernelTCP": 11534336
    },
EOF
)
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    CGROUP_MEMORY_ROOT=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'MEMORY'\>/ { print $5; exit }')
    CGROUP_MEMORY="${CGROUP_MEMORY_ROOT}/runc-update-integration-test"

    # check that initial values were properly set
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 16777216
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 11534336

    # update kernel memory limit
    runc update test_update --kernel-memory 50331648
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 50331648

    # update kernel memory tcp limit
    runc update test_update --kernel-memory-tcp 41943040
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 41943040

    # Get unlimited kernel memory value
    UNLIMITED_MEMORY=$(cat "${CGROUP_MEMORY_ROOT}/memory.kmem.limit_in_bytes")

    # try to remove kernel memory limit
    runc update test_update --kernel-memory -1
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" ${UNLIMITED_MEMORY}

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "memory": {
    "kernel": 16777216,
    "kernelTCP": 11534336
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 16777216
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 11534336

    # redo all the changes at once
    runc update test_update --kernel-memory 50331648 --kernel-memory-tcp 41943040
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 50331648
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 41943040

    # reset to initial test value via json file
    DATA=$(cat <<"EOF"
{
  "memory": {
    "kernel": 16777216,
    "kernelTCP": 11534336
  }
}
EOF
)
    echo $DATA > $BATS_TMPDIR/runc-update-integration-test.json

    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.limit_in_bytes" 16777216
    check_cgroup_value $CGROUP_MEMORY "memory.kmem.tcp.limit_in_bytes" 11534336
}

@test "runc update cpu" {
    requires root

    DATA=$(cat <<EOF
    "cpu": {
        "shares": 100,
        "quota": 500000,
        "period": 1000000
    },
EOF
)
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    CGROUP_CPU_ROOT=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'CPU'\>/ { print $5; exit }')
    CGROUP_CPU="${CGROUP_CPU_ROOT}/runc-update-integration-test"

    # check that initial values were properly set
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 1000000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 500000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 100

    # update cpu-period
    runc update test_update --cpu-period 900000
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 900000

    # update cpu-quota
    runc update test_update --cpu-quota 600000
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 600000

    # update cpu-shares
    runc update test_update --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.shares" 200

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 1000000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 500000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 100

    # redo all the changes at once
    runc update test_update --cpu-period 900000 --cpu-quota 600000 --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 900000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 600000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 200

    # reset to initial test value via json file
    DATA=$(cat <<"EOF"
{
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000
  }
}
EOF
)
    echo $DATA > $BATS_TMPDIR/runc-update-integration-test.json

    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPU "cpu.cfs_period_us" 1000000
    check_cgroup_value $CGROUP_CPU "cpu.cfs_quota_us" 500000
    check_cgroup_value $CGROUP_CPU "cpu.shares" 100
}

@test "runc update cpuset" {
    requires root multi_cores

    DATA=$(cat <<EOF
    "cpu": {
        "cpus": "0"
    },
EOF
)
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    CGROUP_CPUSET_ROOT=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'CPUSET'\>/ { print $5; exit }')
    CGROUP_CPUSET="${CGROUP_CPUSET_ROOT}/runc-update-integration-test"

    # check that initial values were properly set
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 0

    runc update test_update --cpuset-cpus "1"
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 1

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "cpu": {
    "cpus": "0"
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 0

    # redo all the changes at once
    runc update test_update --cpuset-cpus 1
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 1

    # reset to initial test value via json file
    DATA=$(cat <<"EOF"
{
  "cpu": {
    "cpus": "0"
  }
}
EOF
)
    echo $DATA > $BATS_TMPDIR/runc-update-integration-test.json
    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_CPUSET "cpuset.cpus" 0
}

@test "runc update blkio" {
    requires root

    DATA=$(cat <<EOF
    "blockio": {
        "weight": 1000
    },
EOF
)
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    CGROUP_BLKIO_ROOT=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'BLKIO'\>/ { print $5; exit }')
    CGROUP_BLKIO="${CGROUP_BLKIO_ROOT}/runc-update-integration-test"

    # check that initial values were properly set
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 1000

    # update blkio-weight
    runc update test_update --blkio-weight 500
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 500

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "blockIO": {
    "weight": 1000
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 1000

    # redo all the changes at once
    runc update test_update --blkio-weight 500
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 500

    # reset to initial test value via json file
    DATA=$(cat <<"EOF"
{
  "blockIO": {
    "weight": 1000
  }
}
EOF
)
    echo $DATA > $BATS_TMPDIR/runc-update-integration-test.json
    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_BLKIO "blkio.weight" 1000
}

@test "runc update pids" {
    requires root cgroups_pids

    DATA=$(cat <<EOF
    "pids": {
        "limit": 20
    },
EOF
)
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    CGROUP_PIDS_ROOT=$(grep "cgroup"  /proc/self/mountinfo | gawk 'toupper($NF) ~ /\<'PIDS'\>/ { print $5; exit }')
    CGROUP_PIDS="${CGROUP_PIDS_ROOT}/runc-update-integration-test"

    # check that initial values were properly set
    check_cgroup_value $CGROUP_PIDS "pids.max" 20

    # update pids limit
    runc update test_update --pids-limit 10
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_PIDS "pids.max" 10

    # Revert to the test initial value via json on stding
    runc update  -r - test_update <<EOF
{
  "pids": {
    "limit": 20
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_PIDS "pids.max" 20

    # redo all the changes at once
    runc update test_update --pids-limit 10
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_PIDS "pids.max" 10

    # reset to initial test value via json file
    DATA=$(cat <<"EOF"
{
  "pids": {
    "limit": 20
  }
}
EOF
)
    echo $DATA > $BATS_TMPDIR/runc-update-integration-test.json
    runc update  -r $BATS_TMPDIR/runc-update-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value $CGROUP_PIDS "pids.max" 20
}

@test "update rt period and runtime" {
    requires cgroups_rt

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update_rt
    [ "$status" -eq 0 ]

    # get the cgroup paths
    eval CGROUP_CPU="${CGROUP_CPU_BASE_PATH}/runc-update-integration-test"

    runc update  -r - test_update_rt <<EOF
{
  "cpu": {
    "realtimePeriod": 800001,
    "realtimeRuntime": 500001
  }
}
EOF
    check_cgroup_value $CGROUP_CPU "cpu.rt_period_us" 800001
    check_cgroup_value $CGROUP_CPU "cpu.rt_runtime_us" 500001

    runc update test_update_rt --cpu-rt-period 900001 --cpu-rt-runtime 600001

    check_cgroup_value $CGROUP_CPU "cpu.rt_period_us" 900001
    check_cgroup_value $CGROUP_CPU "cpu.rt_runtime_us" 600001
}

# TODO: test rt cgroup updating
