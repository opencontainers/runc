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
    update_config 	' .linux.resources.memory |= {"limit": 33554432, "reservation": 25165824}
			| .linux.resources.cpu |= {"shares": 100, "quota": 500000, "period": 1000000, "cpus": "0"}
			| .linux.resources.pids |= {"limit": 20}' ${BUSYBOX_BUNDLE}
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
        HAVE_SWAP="no"
        if [ -f "${CGROUP_MEMORY_BASE_PATH}/${MEM_SWAP}" ]; then
            HAVE_SWAP="yes"
        fi
        ;;
    yes)
        MEM_LIMIT="memory.max"
        SD_MEM_LIMIT="MemoryMax"
        MEM_RESERVE="memory.low"
        SD_MEM_RESERVE="MemoryLow"
        MEM_SWAP="memory.swap.max"
        SD_MEM_SWAP="MemorySwapMax"
        SYSTEM_MEM="max"
        HAVE_SWAP="yes"
        ;;
    esac
    SD_UNLIMITED="infinity"
    SD_VERSION=$(systemctl --version | awk '{print $2; exit}')
    if [ $SD_VERSION -lt 227 ]; then
        SD_UNLIMITED="18446744073709551615"
    fi

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
    if [ "$HAVE_SWAP" = "yes" ]; then
        # try to remove memory swap limit
        runc update test_update --memory-swap -1
        [ "$status" -eq 0 ]
        check_cgroup_value "$MEM_SWAP" $SYSTEM_MEM
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
    if [ "$HAVE_SWAP" = "yes" ]; then
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

function check_cpu_quota() {
    local quota=$1
    local period=$2
    local sd_quota=$3

    if [ "$CGROUP_UNIFIED" = "yes" ]; then
        if [ "$quota" = "-1" ]; then
            quota="max"
        fi
        check_cgroup_value "cpu.max" "$quota $period"
    else
        check_cgroup_value "cpu.cfs_quota_us" $quota
        check_cgroup_value "cpu.cfs_period_us" $period
    fi
    # systemd values are the same for v1 and v2
    check_systemd_value "CPUQuotaPerSecUSec" $sd_quota

    # CPUQuotaPeriodUSec requires systemd >= v242
    [ "$(systemd_version)" -lt 242 ] && return

    local sd_period=$(( period/1000 ))ms
    [ "$sd_period" = "1000ms" ] && sd_period="1s"
    local sd_infinity=""
    # 100ms is the default value, and if not set, shown as infinity
    [ "$sd_period" = "100ms" ] && sd_infinity="infinity"
    check_systemd_value "CPUQuotaPeriodUSec" $sd_period $sd_infinity
}

function check_cpu_shares() {
    local shares=$1

    if [ "$CGROUP_UNIFIED" = "yes" ]; then
        local weight=$((1 + ((shares - 2) * 9999) / 262142))
        check_cgroup_value "cpu.weight" $weight
        check_systemd_value "CPUWeight" $weight
    else
        check_cgroup_value "cpu.shares" $shares
        check_systemd_value "CPUShares" $shares
    fi
}

@test "update cgroup cpu limits" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

    # run a few busyboxes detached
    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    # check that initial values were properly set
    check_cpu_quota 500000 1000000 "500ms"
    check_cpu_shares 100

    # update cpu period
    runc update test_update --cpu-period 900000
    [ "$status" -eq 0 ]
    check_cpu_quota 500000 900000 "560ms"

    # update cpu quota
    runc update test_update --cpu-quota 600000
    [ "$status" -eq 0 ]
    check_cpu_quota 600000 900000 "670ms"

    # remove cpu quota
    runc update test_update --cpu-quota -1
    [ "$status" -eq 0 ]
    check_cpu_quota -1 900000 "infinity"

    # update cpu-shares
    runc update test_update --cpu-share 200
    [ "$status" -eq 0 ]
    check_cpu_shares 200

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
    check_cpu_quota 500000 1000000 "500ms"

    # redo all the changes at once
    runc update test_update \
        --cpu-period 900000 --cpu-quota 600000 --cpu-share 200
    [ "$status" -eq 0 ]
    check_cpu_quota 600000 900000 "670ms"
    check_cpu_shares 200

    # remove cpu quota and reset the period
    runc update test_update --cpu-quota -1 --cpu-period 100000
    [ "$status" -eq 0 ]
    check_cpu_quota -1 100000 "infinity"

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
    check_cpu_quota 500000 1000000 "500ms"
    check_cpu_shares 100
}

@test "set cpu period with no quota" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

    update_config '.linux.resources.cpu |= { "period": 1000000 }' ${BUSYBOX_BUNDLE}

    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    check_cpu_quota -1 1000000 "infinity"
}

@test "set cpu quota with no period" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

    update_config '.linux.resources.cpu |= { "quota": 5000 }' ${BUSYBOX_BUNDLE}

    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]
    check_cpu_quota 5000 100000 "50ms"
}

@test "update cpu period with no previous period/quota set" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

    update_config '.linux.resources.cpu |= {}' ${BUSYBOX_BUNDLE}

    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    # update the period alone, no old values were set
    runc update --cpu-period 50000 test_update
    [ "$status" -eq 0 ]
    check_cpu_quota -1 50000 "infinity"
}

@test "update cpu quota with no previous period/quota set" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

    update_config '.linux.resources.cpu |= {}' ${BUSYBOX_BUNDLE}

    runc run -d --console-socket $CONSOLE_SOCKET test_update
    [ "$status" -eq 0 ]

    # update the quota alone, no old values were set
    runc update --cpu-quota 30000 test_update
    [ "$status" -eq 0 ]
    check_cpu_quota 30000 100000 "300ms"
}

@test "update rt period and runtime" {
    [[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup
    requires cgroups_v1 cgroups_rt no_systemd

    # By default, "${CGROUP_CPU}/cpu.rt_runtime_us" is set to 0, which inhibits
    # setting the container's realtimeRuntime. (#2046)
    #
    # When $CGROUP_CPU is "/sys/fs/cgroup/cpu,cpuacct/runc-cgroups-integration-test/test-cgroup",
    # we write the values of /sys/fs/cgroup/cpu,cpuacct/cpu.rt_{period,runtime}_us to:
    # - sys/fs/cgroup/cpu,cpuacct/runc-cgroups-integration-test/cpu.rt_{period,runtime}_us
    # - sys/fs/cgroup/cpu,cpuacct/runc-cgroups-integration-test/test-cgroup/cpu.rt_{period,runtime}_us
    #
    # Typically period=1000000 runtime=950000 .
    #
    # TODO: support systemd
    mkdir -p "$CGROUP_CPU"
    local root_period=$(cat "${CGROUP_CPU_BASE_PATH}/cpu.rt_period_us")
    local root_runtime=$(cat "${CGROUP_CPU_BASE_PATH}/cpu.rt_runtime_us")
    # the following IFS magic sets dirs=("runc-cgroups-integration-test" "test-cgroup")
    IFS='/' read -r -a dirs <<< $(echo ${CGROUP_CPU} | sed -e s@^${CGROUP_CPU_BASE_PATH}/@@)
    for (( i = 0; i < ${#dirs[@]}; i++ )); do
        local target="$CGROUP_CPU_BASE_PATH"
        for (( j = 0; j <= i; j++ )); do
            target="${target}/${dirs[$j]}"
        done
        target_period="${target}/cpu.rt_period_us"
        echo "Writing ${root_period} to ${target_period}"
        echo "$root_period" > "$target_period"
        target_runtime="${target}/cpu.rt_runtime_us"
        echo "Writing ${root_runtime} to ${target_runtime}"
        echo "$root_runtime" > "$target_runtime"
    done

    # run a detached busybox
    runc run -d --console-socket $CONSOLE_SOCKET test_update_rt
    [ "$status" -eq 0 ]

    runc update  -r - test_update_rt <<EOF
{
  "cpu": {
    "realtimeRuntime": 500001
  }
}
EOF
    check_cgroup_value "cpu.rt_period_us" "$root_period"
    check_cgroup_value "cpu.rt_runtime_us" 500001

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
    update_config '.process.args |= ["sh", "-c", "while true; do echo >/dev/null; done"]' 

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
