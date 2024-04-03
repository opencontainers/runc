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

	[[ "$ROOTLESS" -ne 0 ]] && requires rootless_cgroup

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

@test "runc run [joining existing container namespaces]" {
	# Create a detached container with the namespaces we want. We notably want
	# to include userns, which requires config-related configuration.
	if [ $EUID -eq 0 ]; then
		update_config '.linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 100}]
			| .linux.gidMappings += [{"containerID": 0, "hostID": 200000, "size": 200}]'
		mkdir -p rootfs/{proc,sys,tmp}
	fi
	update_config '.process.args = ["sleep", "infinity"]'

	runc run -d --console-socket "$CONSOLE_SOCKET" target_ctr
	[ "$status" -eq 0 ]

	# Modify our container's configuration such that it is just going to
	# inherit all of the namespaces of the target container.
	#
	# NOTE: We cannot join the mount namespace of another container because of
	# some quirks of the runtime-spec. In particular, we MUST pivot_root into
	# root.path and root.path MUST be set in the config, so runc cannot just
	# ignore root.path when joining namespaces (and root.path doesn't exist
	# inside root.path, for obvious reasons).
	#
	# We could hack around this (create a copy of the rootfs inside the rootfs,
	# or use a simpler mount namespace target), but those wouldn't be similar
	# tests to the other namespace joining tests.
	target_pid="$(__runc state target_ctr | jq .pid)"
	update_config '.linux.namespaces |= map_values(.path = if .type == "mount" then "" else "/proc/'"$target_pid"'/ns/" + ({"network": "net", "mount": "mnt"}[.type] // .type) end)'
	# Remove the userns configuration (it cannot be changed).
	update_config '.linux |= (del(.uidMappings) | del(.gidMappings))'

	runc run -d --console-socket "$CONSOLE_SOCKET" attached_ctr
	[ "$status" -eq 0 ]

	# Make sure there are two sleep processes in our container.
	runc exec attached_ctr ps aux
	[ "$status" -eq 0 ]
	run -0 grep "sleep infinity" <<<"$output"
	[ "${#lines[@]}" -eq 2 ]

	# ... that the userns mappings are the same...
	runc exec attached_ctr cat /proc/self/uid_map
	[ "$status" -eq 0 ]
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+100000\s+100$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi
	runc exec attached_ctr cat /proc/self/gid_map
	[ "$status" -eq 0 ]
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+200000\s+200$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi
}
