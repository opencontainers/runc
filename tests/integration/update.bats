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
    }
EOF
    )
    DATA=$(echo ${DATA} | sed 's/\n/\\n/g')
    if grep -qw \"resources\" ${BUSYBOX_BUNDLE}/config.json; then
        sed -i "s/\(\"resources\": {\)/\1\n${DATA},/" ${BUSYBOX_BUNDLE}/config.json
    else
        sed -i "s/\(\"linux\": {\)/\1\n\"resources\": {${DATA}},/" ${BUSYBOX_BUNDLE}/config.json
    fi
}

# Tests whatever limits are (more or less) common between cgroup
# v1 and v2: memory/swap, pids, and cpuset.
@test "update cgroup v1/v2 common limits" {
    [[ "$ROOTLESS" -ne 0 && -z "$RUNC_USE_SYSTEMD" ]] && requires rootless_cgroup
    if [[ "$ROOTLESS" -ne 0 && -n "$RUNC_USE_SYSTEMD" ]]; then
        file="/sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers"
        # NOTE: delegation of cpuset requires systemd >= 244 (Fedora >= 32, Ubuntu >= 20.04).
        for f in memory pids cpuset; do
            if grep -qwv $f $file; then
                skip "$f is not enabled in $file"
            fi
        done
    fi
    init_cgroup_paths

    # run a few busyboxes detached
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    # Set a few variables to make the code below work for both v1 and v2
    case $CGROUP_UNIFIED in
    no)
        MEM_LIMIT="memory.limit_in_bytes"
        SD_MEM_LIMIT="MemoryLimit"
        MEM_RESERVE="memory.soft_limit_in_bytes"
        SD_MEM_RESERVE="unsupported"
        MEM_SWAP="memory.memsw.limit_in_bytes"
        SD_MEM_SWAP="unsupported"
        SYSTEM_MEM=$(cat "${CGROUP_MEMORY_BASE_PATH}/${MEM_LIMIT}")
        SYSTEM_MEM_SWAP=$(cat "${CGROUP_MEMORY_BASE_PATH}/$MEM_SWAP")
        ;;
    yes)
        MEM_LIMIT="memory.max"
        SD_MEM_LIMIT="MemoryMax"
        MEM_RESERVE="memory.low"
        SD_MEM_RESERVE="MemoryLow"
        MEM_SWAP="memory.swap.max"
        SD_MEM_SWAP="MemorySwapMax"
        SYSTEM_MEM="max"
        SYSTEM_MEM_SWAP="max"
        CGROUP_MEMORY=$CGROUP_PATH
        ;;
    esac
    SD_UNLIMITED="infinity"

    # check that initial values were properly set
    check_cgroup_value "cpuset.cpus" 0
    if [[ "$CGROUP_UNIFIED" = "yes" ]] && ! grep -qw memory "$CGROUP_PATH/cgroup.controllers"; then
    # This happen on containerized environment because "echo +memory > /sys/fs/cgroup/cgroup.subtree_control" fails with EINVAL
        skip "memory controller not available"
    fi
    check_cgroup_value $MEM_LIMIT 33554432
    check_systemd_value $SD_MEM_LIMIT 33554432

    check_cgroup_value $MEM_RESERVE 25165824
    check_systemd_value $SD_MEM_RESERVE 25165824

    check_cgroup_value "pids.max" 20
    check_systemd_value "TasksMax" 20

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
    check_systemd_value $SD_MEM_LIMIT 67108864

    runc update test_update --memory 50M
    [ "$status" -eq 0 ]
    check_cgroup_value $MEM_LIMIT 52428800
    check_systemd_value $SD_MEM_LIMIT 52428800

    # update memory soft limit
    runc update test_update --memory-reservation 33554432
    [ "$status" -eq 0 ]
    check_cgroup_value "$MEM_RESERVE" 33554432
    check_systemd_value "$SD_MEM_RESERVE" 33554432

    # Run swap memory tests if swap is available
    if [ -f "$CGROUP_MEMORY/$MEM_SWAP" ]; then
        # try to remove memory swap limit
        runc update test_update --memory-swap -1
        [ "$status" -eq 0 ]
        check_cgroup_value "$MEM_SWAP" $SYSTEM_MEM_SWAP
        check_systemd_value "$SD_MEM_SWAP" $SD_UNLIMITED

        # update memory swap
        if [ "$CGROUP_UNIFIED" = "yes" ]; then
            # for cgroupv2, memory and swap can only be set together
            runc update test_update --memory 52428800 --memory-swap 96468992
            [ "$status" -eq 0 ]
            # for cgroupv2, swap is a separate limit (it does not include mem)
            check_cgroup_value "$MEM_SWAP" $((96468992-52428800))
            check_systemd_value "$SD_MEM_SWAP" $((96468992-52428800))
        else
            runc update test_update --memory-swap 96468992
            [ "$status" -eq 0 ]
            check_cgroup_value "$MEM_SWAP" 96468992
            check_systemd_value "$SD_MEM_SWAP" 96468992
        fi
    fi

    # try to remove memory limit
    runc update test_update --memory -1
    [ "$status" -eq 0 ]

    # check memory limit is gone
    check_cgroup_value $MEM_LIMIT $SYSTEM_MEM
    check_systemd_value $SD_MEM_LIMIT $SD_UNLIMITED

    # check swap memory limited is gone
    if [ -f "$CGROUP_MEMORY/$MEM_SWAP" ]; then
        check_cgroup_value $MEM_SWAP $SYSTEM_MEM
        check_systemd_value "$SD_MEM_SWAP" $SD_UNLIMITED
    fi

    # update pids limit
    runc update test_update --pids-limit 10
    [ "$status" -eq 0 ]
    check_cgroup_value "pids.max" 10
    check_systemd_value "TasksMax" 10

    # unlimited
    runc update test_update --pids-limit -1
    [ "$status" -eq 0 ]
    check_cgroup_value "pids.max" max
    check_systemd_value "TasksMax" $SD_UNLIMITED

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
    check_systemd_value $SD_MEM_LIMIT 33554432

    check_cgroup_value $MEM_RESERVE 25165824
    check_systemd_value $SD_MEM_RESERVE 25165824

    check_cgroup_value "pids.max" 20
    check_systemd_value "TasksMax" 20

    # redo all the changes at once
    runc update test_update \
        --cpu-period 900000 --cpu-quota 600000 --cpu-share 200 \
        --memory 67108864 --memory-reservation 33554432 \
        --pids-limit 10
    [ "$status" -eq 0 ]
    check_cgroup_value $MEM_LIMIT 67108864
    check_systemd_value $SD_MEM_LIMIT 67108864

    check_cgroup_value $MEM_RESERVE 33554432
    check_systemd_value $SD_MEM_RESERVE 33554432

    check_cgroup_value "pids.max" 10
    check_systemd_value "TasksMax" 10

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
    check_systemd_value $SD_MEM_LIMIT 33554432

    check_cgroup_value $MEM_RESERVE 25165824
    check_systemd_value $SD_MEM_RESERVE 25165824

    check_cgroup_value "pids.max" 20
    check_systemd_value "TasksMax" 20
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
    check_systemd_value "CPUQuotaPerSecUSec" 500ms

    check_cgroup_value "cpu.shares" 100
    check_systemd_value "CPUShares" 100

    # updating cpu period alone is not allowed
    runc update test_update --cpu-period 900000
    [ "$status" -eq 1 ]

    # update cpu quota
    runc update test_update --cpu-quota 600000
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_quota_us" 600000
    check_systemd_value "CPUQuotaPerSecUSec" 600ms

    # update cpu quota and period together
    runc update test_update --cpu-period 900000 --cpu-quota 600000
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_period_us" 900000
    check_cgroup_value "cpu.cfs_quota_us" 600000
    check_systemd_value "CPUQuotaPerSecUSec" 670ms

    # update cpu-shares
    runc update test_update --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.shares" 200
    check_systemd_value "CPUShares" 200

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
    check_systemd_value "CPUQuotaPerSecUSec" 500ms

    check_cgroup_value "cpu.shares" 100
    check_systemd_value "CPUShares" 100

    # redo all the changes at once
    runc update test_update \
        --cpu-period 900000 --cpu-quota 600000 --cpu-share 200
    [ "$status" -eq 0 ]
    check_cgroup_value "cpu.cfs_period_us" 900000
    check_cgroup_value "cpu.cfs_quota_us" 600000
    check_systemd_value "CPUQuotaPerSecUSec" 670ms

    check_cgroup_value "cpu.shares" 200
    check_systemd_value "CPUShares" 200

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
    check_systemd_value "CPUQuotaPerSecUSec" 500ms

    check_cgroup_value "cpu.shares" 100
    check_systemd_value "CPUShares" 100
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

@test "update devices [minimal transition rules]" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
    # This test currently only makes sense on cgroupv1.
    requires cgroups_v1

    # Run a basic shell script that tries to write to /dev/null. If "runc
    # update" makes use of minimal transition rules, updates should not cause
    # writes to fail at any point.
    jq '.process.args = ["sh", "-c", "while true; do echo >/dev/null; done"]' config.json > config.json.tmp
    mv config.json{.tmp,}

    # Set up a temporary console socket and recvtty so we can get the stdio.
    TMP_RECVTTY_DIR="$(mktemp -d "$BATS_TMPDIR/runc-tmp-recvtty.XXXXXX")"
    TMP_RECVTTY_PID="$TMP_RECVTTY_DIR/recvtty.pid"
    TMP_CONSOLE_SOCKET="$TMP_RECVTTY_DIR/console.sock"
    CONTAINER_OUTPUT="$TMP_RECVTTY_DIR/output"
    ("$RECVTTY" --no-stdin --pid-file "$TMP_RECVTTY_PID" \
                --mode single "$TMP_CONSOLE_SOCKET" &>"$CONTAINER_OUTPUT" ) &
    retry 10 0.1 [ -e "$TMP_CONSOLE_SOCKET" ]

    # Run the container in the background.
    runc run -d --console-socket "$TMP_CONSOLE_SOCKET" test_update
    cat "$CONTAINER_OUTPUT"
    [ "$status" -eq 0 ]

    # Trigger an update. This update doesn't actually change the device rules,
    # but it will trigger the devices cgroup code to reapply the current rules.
    # We trigger the update a few times to make sure we hit the race.
    for _ in {1..12}
    do
        # TODO: Update "runc update" so we can change the device rules.
        runc update --pids-limit 30 test_update
        [ "$status" -eq 0 ]
    done

    # Kill recvtty.
    kill -9 "$(<"$TMP_RECVTTY_PID")"

    # There should've been no output from the container.
    cat "$CONTAINER_OUTPUT"
    [ -z "$(<"$CONTAINER_OUTPUT")" ]
}

@test "update paused container" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
    requires cgroups_freezer

    # Run the container in the background.
    runc run -d --console-socket "$CONSOLE_SOCKET" test_update
    [ "$status" -eq 0 ]

    # Pause the container.
    runc pause test_update
    [ "$status" -eq 0 ]

    # Trigger an unrelated update.
    runc update --pids-limit 30 test_update
    [ "$status" -eq 0 ]

    # The container should still be paused.
    testcontainer test_update paused

    # Resume the container.
    runc resume test_update
    [ "$status" -eq 0 ]
}
