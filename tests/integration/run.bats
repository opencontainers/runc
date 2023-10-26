#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/echo", "Hello World"]'
}

function teardown() {
	teardown_bundle
}

@test "runc run" {
	runc run test_hello
	[ "$status" -eq 0 ]

	runc state test_hello
	[ "$status" -ne 0 ]
}

@test "runc run --keep" {
	runc run --keep test_run_keep
	[ "$status" -eq 0 ]

	testcontainer test_run_keep stopped

	runc state test_run_keep
	[ "$status" -eq 0 ]

	runc delete test_run_keep

	runc state test_run_keep
	[ "$status" -ne 0 ]
}

@test "runc run --keep (check cgroup exists)" {
	# for systemd driver, the unit's cgroup path will be auto removed if container's all processes exited
	requires no_systemd
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc run --keep test_run_keep
	[ "$status" -eq 0 ]

	testcontainer test_run_keep stopped

	runc state test_run_keep
	[ "$status" -eq 0 ]

	# check that cgroup exists
	check_cgroup_value "pids.max" "max"

	runc delete test_run_keep

	runc state test_run_keep
	[ "$status" -ne 0 ]
}

@test "runc run [hostname domainname]" {
	update_config ' .process.args |= ["sh"]
			| .hostname = "myhostname"
			| .domainname= "mydomainname"'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_utc
	[ "$status" -eq 0 ]

	# test hostname
	runc exec test_utc hostname
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *'myhostname'* ]]

	# test domainname
	runc exec test_utc cat /proc/sys/kernel/domainname
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *'mydomainname'* ]]
}

# https://github.com/opencontainers/runc/issues/3952
@test "runc run with tmpfs" {
	requires root

	chmod 'a=rwx,ug+s,+t' rootfs/tmp # set all bits
	mode=$(stat -c %A rootfs/tmp)

	# shellcheck disable=SC2016
	update_config '.process.args = ["sh", "-c", "stat -c %A /tmp"]'
	update_config '.mounts += [{"destination": "/tmp", "type": "tmpfs", "source": "tmpfs", "options":["noexec","nosuid","nodev","rprivate"]}]'

	runc run test_tmpfs
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "$mode" ]
}

@test "runc run with tmpfs perms" {
	# shellcheck disable=SC2016
	update_config '.process.args = ["sh", "-c", "stat -c %a /tmp/test"]'
	update_config '.mounts += [{"destination": "/tmp/test", "type": "tmpfs", "source": "tmpfs", "options": ["mode=0444"]}]'

	# Directory is to be created by runc.
	runc run test_tmpfs
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "444" ]

	# Run a 2nd time with the pre-existing directory.
	# Ref: https://github.com/opencontainers/runc/issues/3911
	runc run test_tmpfs
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "444" ]

	# Existing directory, custom perms, no mode on the mount,
	# so it should use the directory's perms.
	update_config '.mounts[-1].options = []'
	chmod 0710 rootfs/tmp/test
	# shellcheck disable=SC2016
	runc run test_tmpfs
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "710" ]

	# Add back the mode on the mount, and it should use that instead.
	# Just for fun, use different perms than was used earlier.
	# shellcheck disable=SC2016
	update_config '.mounts[-1].options = ["mode=0410"]'
	runc run test_tmpfs
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "410" ]
}

@test "runc run [runc-dmz]" {
	runc --debug run test_hello
	[ "$status" -eq 0 ]
	[[ "$output" = *"Hello World"* ]]
	# We use runc-dmz if we can.
	[[ "$output" = *"runc-dmz: using runc-dmz"* ]]
}

@test "runc run [cap_sys_ptrace -> /proc/self/exe clone]" {
	# Add CAP_SYS_PTRACE to the bounding set, the minimum needed to indicate a
	# container process _could_ get CAP_SYS_PTRACE.
	update_config '.process.capabilities.bounding += ["CAP_SYS_PTRACE"]'

	runc --debug run test_hello
	[ "$status" -eq 0 ]
	[[ "$output" = *"Hello World"* ]]
	if [ "$EUID" -ne 0 ] && is_kernel_gte 4.10; then
		# For Linux 4.10 and later, rootless containers will use runc-dmz
		# because they are running in a user namespace. See isDmzBinarySafe().
		[[ "$output" = *"runc-dmz: using runc-dmz"* ]]
	else
		# If the container has CAP_SYS_PTRACE and is not rootless, we use
		# /proc/self/exe cloning.
		[[ "$output" = *"runc-dmz: using /proc/self/exe clone"* ]]
	fi
}

@test "RUNC_DMZ=legacy runc run [/proc/self/exe clone]" {
	RUNC_DMZ=legacy runc --debug run test_hello
	[ "$status" -eq 0 ]
	[[ "$output" = *"Hello World"* ]]
	[[ "$output" = *"runc-dmz: using /proc/self/exe clone"* ]]
}
