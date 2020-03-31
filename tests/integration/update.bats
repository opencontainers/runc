#!/usr/bin/env bats

load helpers

function teardown() {
    rm -f $BATS_TMPDIR/runc-cgroups-integration-test.json
    teardown_running_container test_update
    teardown_running_container test_update_rt
    teardown_busybox
}

function setup() {
    teardown
    setup_busybox

    set_cgroups_path "$BUSYBOX_BUNDLE"

    # Set some initial known values
    DATA=$(cat <<EOF
    "memory": {
        "limit": 33554432,
        "reservation": 25165824
    },
    "cpu": {
        "shares": 100,
        "quota": 500000,
        "period": 1000000,
        "cpus": "0"
    },
    "pids": {
        "limit": 20
    },
EOF
    )
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    sed -i "s/\(\"resources\": {\)/\1\n${DATA}/" ${BUSYBOX_BUNDLE}/config.json
}

# Tests whatever limits are (more or less) common between cgroup
# v1 and v2: memory/swap, pids, and cpuset.
@test "update cgroup v1/v2 common limits" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
    init_cgroup_paths

    # run a few busyboxes detached
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    # Set a few variables to make the code below work for both v1 and v2
    case $CGROUP_UNIFIED in
    no)
        MEM_LIMIT="memory.limit_in_bytes"
        MEM_RESERVE="memory.soft_limit_in_bytes"
        MEM_SWAP="memory.memsw.limit_in_bytes"
        SYSTEM_MEM=$(cat "${CGROUP_MEMORY_BASE_PATH}/${MEM_LIMIT}")
        SYSTEM_MEM_SWAP=$(cat "${CGROUP_MEMORY_BASE_PATH}/$MEM_SWAP")
        ;;
    yes)
        MEM_LIMIT="memory.max"
        MEM_RESERVE="memory.low"
        MEM_SWAP="memory.swap.max"
        SYSTEM_MEM="max"
        SYSTEM_MEM_SWAP="max"
        CGROUP_MEMORY=$CGROUP_PATH
        ;;
    esac

    # check that initial values were properly set
    check_cgroup_value "cpuset.cpus" 0
    check_cgroup_value $MEM_LIMIT 33554432
    check_cgroup_value $MEM_RESERVE 25165824
    check_cgroup_value "pids.max" 20

    # update cpuset if supported (i.e. we're running on a multicore cpu)
    cpu_count=$(grep -c '^processor' /proc/cpuinfo)
    if [ $cpu_count -gt 1 ]; then
        runc update test_update --cpuset-cpus "1"
        [ "$status" -eq 0 ]
        check_cgroup_value "cpuset.cpus" 1
    fi

    # update memory limit
    runc update test_update --memory 67108864
    [ "$status" -eq 0 ]
    check_cgroup_value $MEM_LIMIT 67108864

    runc update test_update --memory 50M
    [ "$status" -eq 0 ]
    check_cgroup_value $MEM_LIMIT 52428800

    # update memory soft limit
    runc update test_update --memory-reservation 33554432
    [ "$status" -eq 0 ]
    check_cgroup_value "$MEM_RESERVE" 33554432

    # Run swap memory tests if swap is available
    if [ -f "$CGROUP_MEMORY/$MEM_SWAP" ]; then
        # try to remove memory swap limit
        runc update test_update --memory-swap -1
        [ "$status" -eq 0 ]
        check_cgroup_value "$MEM_SWAP" $SYSTEM_MEM_SWAP

        # update memory swap
        runc update test_update --memory-swap 96468992
        [ "$status" -eq 0 ]
        check_cgroup_value "$MEM_SWAP" 96468992
    fi

    # try to remove memory limit
    runc update test_update --memory -1
    [ "$status" -eq 0 ]

    # check memory limit is gone
    check_cgroup_value $MEM_LIMIT $SYSTEM_MEM

    # check swap memory limited is gone
    if [ -f "$CGROUP_MEMORY/$MEM_SWAP" ]; then
        check_cgroup_value $MEM_SWAP $SYSTEM_MEM
    fi

    # update pids limit
    runc update test_update --pids-limit 10
    [ "$status" -eq 0 ]
    check_cgroup_value "pids.max" 10

    # Revert to the test initial value via json on stdin
    runc update  -r - test_update <<EOF
{
  "memory": {
    "limit": 33554432,
    "reservation": 25165824
  },
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000,
    "cpus": "0"
  },
  "pids": {
    "limit": 20
  }
}
EOF
    [ "$status" -eq 0 ]
    check_cgroup_value "cpuset.cpus" 0
    check_cgroup_value $MEM_LIMIT 33554432
    check_cgroup_value $MEM_RESERVE 25165824
    check_cgroup_value "pids.max" 20

    # redo all the changes at once
    runc update test_update \
        --cpu-period 900000 --cpu-quota 600000 --cpu-share 200 \
        --memory 67108864 --memory-reservation 33554432 \
        --pids-limit 10
    [ "$status" -eq 0 ]
    check_cgroup_value $MEM_LIMIT 67108864
    check_cgroup_value $MEM_RESERVE 33554432
    check_cgroup_value "pids.max" 10

    # reset to initial test value via json file
    cat << EOF > $BATS_TMPDIR/runc-cgroups-integration-test.json
{
  "memory": {
    "limit": 33554432,
    "reservation": 25165824
  },
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000,
    "cpus": "0"
  },
  "pids": {
    "limit": 20
  }
}
EOF

    runc update  -r $BATS_TMPDIR/runc-cgroups-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value "cpuset.cpus" 0
    check_cgroup_value $MEM_LIMIT 33554432
    check_cgroup_value $MEM_RESERVE 25165824
    check_cgroup_value "pids.max" 20
}

@test "update cgroup v1 cpu limits" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
    requires cgroups_v1

    # run a few busyboxes detached
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    # check that initial values were properly set
    check_cgroup_value "cpu.cfs_period_us" 1000000
    check_cgroup_value "cpu.cfs_quota_us" 500000
    check_cgroup_value "cpu.shares" 100

    # update cpu-period
    runc update test_update --cpu-period 900000
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_period_us" 900000

    # update cpu-quota
    runc update test_update --cpu-quota 600000
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_quota_us" 600000

    # update cpu-shares
    runc update test_update --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.shares" 200

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
    check_cgroup_value "cpu.cfs_period_us" 1000000
    check_cgroup_value "cpu.cfs_quota_us" 500000
    check_cgroup_value "cpu.shares" 100

    # redo all the changes at once
    runc update test_update \
        --cpu-period 900000 --cpu-quota 600000 --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_period_us" 900000
    check_cgroup_value "cpu.cfs_quota_us" 600000
    check_cgroup_value "cpu.shares" 200

    # reset to initial test value via json file
    cat << EOF > $BATS_TMPDIR/runc-cgroups-integration-test.json
{
  "cpu": {
    "shares": 100,
    "quota": 500000,
    "period": 1000000
  }
}
EOF

    runc update  -r $BATS_TMPDIR/runc-cgroups-integration-test.json test_update
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_period_us" 1000000
    check_cgroup_value "cpu.cfs_quota_us" 500000
    check_cgroup_value "cpu.shares" 100
}

@test "update rt period and runtime" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
    requires cgroups_rt

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update_rt
    [ "$status" -eq 0 ]

    runc update  -r - test_update_rt <<EOF
{
  "cpu": {
    "realtimePeriod": 800001,
    "realtimeRuntime": 500001
  }
}
EOF
    check_cgroup_value "cpu.rt_period_us" 800001
    check_cgroup_value "cpu.rt_runtime_us" 500001

    runc update test_update_rt --cpu-rt-period 900001 --cpu-rt-runtime 600001

    check_cgroup_value "cpu.rt_period_us" 900001
    check_cgroup_value "cpu.rt_runtime_us" 600001
}
