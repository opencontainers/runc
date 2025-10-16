#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc create" {
	runc -0 create --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox created

	runc -0 start test_busybox

	testcontainer test_busybox running
}

@test "runc create exec" {
	runc -0 create --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox created

	runc -0 exec test_busybox true

	testcontainer test_busybox created

	runc -0 start test_busybox

	testcontainer test_busybox running
}

@test "runc create --pid-file" {
	runc -0 create --pid-file pid.txt --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox created

	[ -e pid.txt ]
	[[ $(cat pid.txt) = $(__runc state test_busybox | jq '.pid') ]]

	runc -0 start test_busybox

	testcontainer test_busybox running
}

@test "runc create --pid-file with new CWD" {
	bundle="$(pwd)"
	mkdir pid_file
	cd pid_file

	runc -0 create --pid-file pid.txt -b "$bundle" --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox created

	[ -e pid.txt ]
	[[ $(cat pid.txt) = $(__runc state test_busybox | jq '.pid') ]]

	runc -0 start test_busybox

	testcontainer test_busybox running
}

# https://github.com/opencontainers/runc/issues/4394#issuecomment-2334926257
@test "runc create [shared pidns + rootless]" {
	# Remove pidns so it's shared with the host.
	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	if [ $EUID -ne 0 ]; then
		if rootless_cgroup; then
			# Rootless containers have empty cgroup path by default.
			set_cgroups_path
		fi
		# Can't mount real /proc when rootless + no pidns,
		# so change it to a bind-mounted one from the host.
		update_config '	  .mounts |= map((select(.type == "proc")
                                | .type = "none"
                                | .source = "/proc"
                                | .options = ["rbind", "nosuid", "nodev", "noexec"]
                          ) // .)'
	fi

	exp="Such configuration is strongly discouraged"
	runc -0 create --console-socket "$CONSOLE_SOCKET" test
	if [ $EUID -ne 0 ] && ! rootless_cgroup; then
		[[ "$output" = *"$exp"* ]]
	else
		[[ "$output" != *"$exp"* ]]
	fi
}
