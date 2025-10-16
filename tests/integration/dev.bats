#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run [redundant default /dev/tty]" {
	update_config ' .linux.devices += [{"path": "/dev/tty", "type": "c", "major": 5, "minor": 0}]
		      | .process.args |= ["ls", "-lLn", "/dev/tty"]'

	runc -0 run test_dev

	if [ $EUID -ne 0 ]; then
		[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"65534".+"65534".+"5,".+"0".+"/dev/tty" ]]
	else
		[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"0".+"0".+"5,".+"0".+"/dev/tty" ]]
	fi
}

@test "runc run [redundant default /dev/ptmx]" {
	update_config ' .linux.devices += [{"path": "/dev/ptmx", "type": "c", "major": 5, "minor": 2}]
		      | .process.args |= ["ls", "-lLn", "/dev/ptmx"]'

	runc -0 run test_dev
	[[ "${lines[0]}" =~ "crw-rw-rw".+"1".+"0".+"0".+"5,".+"2".+"/dev/ptmx" ]]
}

@test "runc run/update [device cgroup deny]" {
	requires root

	update_config ' .linux.resources.devices = [{"allow": false, "access": "rwm"}]
			| .linux.devices = [{"path": "/dev/kmsg", "type": "c", "major": 1, "minor": 11}]
			| .process.capabilities.bounding += ["CAP_SYSLOG"]
			| .process.capabilities.effective += ["CAP_SYSLOG"]
			| .process.capabilities.permitted += ["CAP_SYSLOG"]
			| .process.args |= ["sh"]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_deny

	# test write
	runc -1 exec test_deny sh -c 'hostname | tee /dev/kmsg'
	[[ "${output}" == *'Operation not permitted'* ]]

	# test read
	runc -1 exec test_deny sh -c 'head -n 1 /dev/kmsg'
	[[ "${output}" == *'Operation not permitted'* ]]

	runc update test_deny --pids-limit 42

	# test write
	runc -1 exec test_deny sh -c 'hostname | tee /dev/kmsg'
	[[ "${output}" == *'Operation not permitted'* ]]

	# test read
	runc -1 exec test_deny sh -c 'head -n 1 /dev/kmsg'
	[[ "${output}" == *'Operation not permitted'* ]]
}

@test "runc run [device cgroup allow rw char device]" {
	requires root

	update_config ' .linux.resources.devices = [{"allow": false, "access": "rwm"},{"allow": true, "type": "c", "major": 1, "minor": 11, "access": "rw"}]
			| .linux.devices = [{"path": "/dev/kmsg", "type": "c", "major": 1, "minor": 11}]
			| .process.args |= ["sh"]
			| .process.capabilities.bounding += ["CAP_SYSLOG"]
			| .process.capabilities.effective += ["CAP_SYSLOG"]
			| .process.capabilities.permitted += ["CAP_SYSLOG"]
			| .hostname = "myhostname"'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_allow_char

	# test write
	runc -0 exec test_allow_char sh -c 'hostname | tee /dev/kmsg'
	[[ "${lines[0]}" == *'myhostname'* ]]

	# test read
	runc -0 exec test_allow_char sh -c 'head -n 1 /dev/kmsg'

	# test access
	TEST_NAME="dev_access_test"
	gcc -static -o "rootfs/bin/${TEST_NAME}" "${TESTDATA}/${TEST_NAME}.c"
	runc -0 exec test_allow_char sh -c "${TEST_NAME} /dev/kmsg"
}

@test "runc run [device cgroup allow rm block device]" {
	requires root

	# Get the first block device.
	IFS=$' \t:' read -r device major minor <<<"$(lsblk -nd -o NAME,MAJ:MIN)"
	# Could have used -o PATH but lsblk from CentOS 7 does not have it.
	device="/dev/$device"

	update_config ' .linux.resources.devices = [{"allow": false, "access": "rwm"},{"allow": true, "type": "b", "major": '"$major"', "minor": '"$minor"', "access": "rwm"}]
			| .linux.devices = [{"path": "'"$device"'", "type": "b", "major": '"$major"', "minor": '"$minor"'}]
			| .process.args |= ["sh"]
			| .process.capabilities.bounding += ["CAP_MKNOD"]
			| .process.capabilities.effective += ["CAP_MKNOD"]
			| .process.capabilities.permitted += ["CAP_MKNOD"]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_allow_block

	# test mknod
	runc -0 exec test_allow_block sh -c 'mknod /dev/fooblock b '"$major"' '"$minor"''

	# test read
	runc -0 exec test_allow_block sh -c 'fdisk -l '"$device"''
}

# https://github.com/opencontainers/runc/issues/3551
@test "runc exec vs systemctl daemon-reload" {
	requires systemd root

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_exec

	runc -0 exec -t test_exec sh -c "ls -l /proc/self/fd/0; echo 123"

	systemctl daemon-reload

	runc -0 exec -t test_exec sh -c "ls -l /proc/self/fd/0; echo 123"
}

# https://github.com/opencontainers/runc/issues/4568
@test "runc run [devices vs systemd NeedDaemonReload]" {
	# The systemd bug is there since v230, see
	# https://github.com/systemd/systemd/pull/3170/commits/ab932a622d57fd327ef95992c343fd4425324088
	# and https://github.com/systemd/systemd/issues/35710.
	requires systemd_v230

	set_cgroups_path
	runc run -d --console-socket "$CONSOLE_SOCKET" test_need_reload
	check_systemd_value "NeedDaemonReload" "no"
}
