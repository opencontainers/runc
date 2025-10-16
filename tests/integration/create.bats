#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# is_allowed_fdtarget checks whether the target of a file descriptor symlink
# conforms to the allowed whitelist.
#
# This whitelist reflects the set of file descriptors that runc legitimately
# opens during container lifecycle operations (e.g., exec, create, and run).
# If runc's internal behavior changes (e.g., new FD types are introduced),
# this function MUST be updated accordingly to avoid false positives.
#
is_allowed_fdtarget() {
	local target="$1"
	{
		# pty devices for stdio
		grep -Ex "/dev/pts/[0-9]+" <<<"$target" ||
			# eventfd, eventpoll, signalfd, etc.
			grep -Ex "anon_inode:\[.+\]" <<<"$target" ||
			# procfs handle cache (pathrs-lite / libpathrs)
			grep -Ex "/(proc)?" <<<"$target" ||
			# anonymous sockets used for IPC
			grep -Ex "socket:\[[0-9]+\]" <<<"$target" ||
			# anonymous pipes used for I/O forwarding
			grep -Ex "pipe:\[[0-9]+\]" <<<"$target" ||
			# "runc start" synchronisation barrier FIFO
			grep -Ex ".*/exec\.fifo" <<<"$target" ||
			# temporary internal fd used in exec.fifo FIFO reopen (pathrs-lite / libpathrs)
			grep -Ex "(/proc)?/1/task/1/fd" <<<"$target" ||
			# overlayfs binary reference (CVE-2019-5736)
			grep -Ex "/runc" <<<"$target" ||
			# memfd cloned binary (CVE-2019-5736)
			grep -Fx "/memfd:runc_cloned:/proc/self/exe (deleted)" <<<"$target"
	} >/dev/null
	return "$?"
}

@test "runc create[detect fd leak as comprehensively as possible]" {
	runc create --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	pid=$(__runc state test_busybox | jq '.pid')
	violation_found=0

	while IFS= read -rd '' link; do
		fd_name=$(basename "$link")
		# Skip . and ..
		if [[ "$fd_name" == "." || "$fd_name" == ".." ]]; then
			continue
		fi

		# Resolve symlink target (use readlink)
		target=$(readlink "$link" 2>/dev/null)
		if [[ -z "$target" ]]; then
			echo "Warning: Cannot read target of $link"
			continue
		fi

		if ! is_allowed_fdtarget "$target"; then
			echo "Violation: FD $fd_name -> '$target'"
			violation_found=1
		fi
	done < <(find "/proc/$pid/fd" -type l -print0)
	[ "$violation_found" -eq 0 ]
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
