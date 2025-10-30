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
	runc -0 run test_hello

	runc ! state test_hello
}

@test "runc run --keep" {
	runc -0 run --keep test_run_keep

	testcontainer test_run_keep stopped

	runc -0 state test_run_keep

	runc -0 delete test_run_keep

	runc ! state test_run_keep
}

@test "runc run --keep (check cgroup exists)" {
	# for systemd driver, the unit's cgroup path will be auto removed if container's all processes exited
	requires no_systemd
	[ $EUID -ne 0 ] && requires rootless_cgroup

	set_cgroups_path

	runc -0 run --keep test_run_keep

	testcontainer test_run_keep stopped

	runc -0 state test_run_keep

	# check that cgroup exists
	check_cgroup_value "pids.max" "max"

	runc -0 delete test_run_keep

	runc ! state test_run_keep
}

@test "runc run [hostname domainname]" {
	update_config ' .process.args |= ["sh"]
			| .hostname = "myhostname"
			| .domainname= "mydomainname"'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_utc

	# test hostname
	runc -0 exec test_utc hostname
	[[ "${lines[0]}" == *'myhostname'* ]]

	# test domainname
	runc -0 exec test_utc cat /proc/sys/kernel/domainname
	[[ "${lines[0]}" == *'mydomainname'* ]]
}

# https://github.com/opencontainers/runc/issues/3952
@test "runc run with tmpfs" {
	requires root

	chmod 'a=rwx,ug+s,+t' rootfs/tmp # set all bits
	mode=$(stat -c %A rootfs/tmp)

	update_config '.process.args = ["sh", "-c", "stat -c %A /tmp"]'
	update_config '.mounts += [{"destination": "/tmp", "type": "tmpfs", "source": "tmpfs", "options":["noexec","nosuid","nodev","rprivate"]}]'

	runc -0 run test_tmpfs
	[ "${lines[0]}" = "$mode" ]
}

@test "runc run with tmpfs perms" {
	update_config '.process.args = ["sh", "-c", "stat -c %a /tmp/test"]'
	update_config '.mounts += [{"destination": "/tmp/test", "type": "tmpfs", "source": "tmpfs", "options": ["mode=0444"]}]'

	# Directory is to be created by runc.
	runc -0 run test_tmpfs
	[ "${lines[0]}" = "444" ]

	# Run a 2nd time with the pre-existing directory.
	# Ref: https://github.com/opencontainers/runc/issues/3911
	runc -0 run test_tmpfs
	[ "${lines[0]}" = "444" ]

	# Existing directory, custom perms, no mode on the mount,
	# so it should use the directory's perms.
	update_config '.mounts[-1].options = []'
	chmod 0710 rootfs/tmp/test
	runc -0 run test_tmpfs
	[ "${lines[0]}" = "710" ]

	# Add back the mode on the mount, and it should use that instead.
	# Just for fun, use different perms than was used earlier.
	update_config '.mounts[-1].options = ["mode=0410"]'
	runc -0 run test_tmpfs
	[ "${lines[0]}" = "410" ]
}

@test "runc run [/proc/self/exe clone]" {
	runc -0 --debug run test_hello
	[[ "$output" = *"Hello World"* ]]
	[[ "$output" = *"runc exeseal: using /proc/self/exe clone"* ]]
	# runc will use fsopen("overlay") if it can.
	if can_fsopen overlay; then
		[[ "$output" = *"runc exeseal: using overlayfs for sealed /proc/self/exe"* ]]
	fi
}

@test "runc run [joining existing container namespaces]" {
	requires timens

	# Create a detached container with the namespaces we want. We notably want
	# to include both userns and timens, which require config-related
	# configuration.
	if [ $EUID -eq 0 ]; then
		update_config '.linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 100}]
			| .linux.gidMappings += [{"containerID": 0, "hostID": 200000, "size": 200}]'
		remap_rootfs
	fi
	update_config '.linux.namespaces += [{"type": "time"}]
		| .linux.timeOffsets = {
			"monotonic": { "secs": 7881, "nanosecs": 2718281 },
			"boottime": { "secs": 1337, "nanosecs": 3141519 }
		}'
	update_config '.process.args = ["sleep", "infinity"]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" target_ctr

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
	# Remove the userns and timens configuration (they cannot be changed).
	update_config '.linux |= (del(.uidMappings) | del(.gidMappings) | del(.timeOffsets))'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" attached_ctr

	# Make sure there are two sleep processes in our container.
	runc -0 exec attached_ctr ps aux
	run -0 grep "sleep infinity" <<<"$output"
	[ "${#lines[@]}" -eq 2 ]

	# ... that the userns mappings are the same...
	runc -0 exec attached_ctr cat /proc/self/uid_map
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+100000\s+100$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi
	runc -0 exec attached_ctr cat /proc/self/gid_map
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+200000\s+200$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi

	# ... as well as the timens offsets.
	runc -0 exec attached_ctr cat /proc/self/timens_offsets
	grep -E '^monotonic\s+7881\s+2718281$' <<<"$output"
	grep -E '^boottime\s+1337\s+3141519$' <<<"$output"
}

@test "runc run [execve error]" {
	cat <<EOF >rootfs/run.sh
#!/mmnnttbb foo bar
sh
EOF
	chmod +x rootfs/run.sh
	update_config '.process.args = [ "/run.sh" ]'
	runc ! run test_hello

	# After the sync socket closed, we should not send error to parent
	# process, or else we will get a unnecessary error log(#4171).
	[ ${#lines[@]} -eq 1 ]
	[[ ${lines[0]} = "exec /run.sh: no such file or directory" ]]
}

# https://github.com/opencontainers/runc/issues/4688
@test "runc run check default home" {
	# cannot start containers as another user in rootless setup without idmap
	[ $EUID -ne 0 ] && requires rootless_idmap
	echo 'tempuser:x:2000:2000:tempuser:/home/tempuser:/bin/sh' >>rootfs/etc/passwd

	# shellcheck disable=SC2016
	update_config '	  .process.cwd = "/root"
			| .process.user.uid = 2000
			| .process.args |= ["sh", "-c", "echo $HOME"]'

	runc -0 run test_busybox
	[ "${lines[0]}" = "/home/tempuser" ]
}
