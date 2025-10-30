#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
test_host_pidns_kill() {
	requires cgroups_freezer

	update_config '	  .linux.namespaces -= [{"type": "pid"}]'
	set_cgroups_path
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		# Can't mount real /proc when rootless + no pidns,
		# so change it to a bind-mounted one from the host.
		update_config '	  .mounts |= map((select(.type == "proc")
					| .type = "none"
					| .source = "/proc"
					| .options = ["rbind", "nosuid", "nodev", "noexec"]
				  ) // .)'
	fi

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	cgpath=$(get_cgroup_path "pids")
	init_pid=$(cat "$cgpath"/cgroup.procs)

	# Start a few more processes.
	for _ in 1 2 3 4 5; do
		__runc exec -d test_busybox sleep 1h
	done

	if [ -v KILL_INIT ]; then
		# Now kill the container's init process. Since the container do
		# not have own PID ns, its init is no special and the container
		# will still be up and running (except for rootless container
		# AND systemd cgroup driver AND systemd > v245, when systemd
		# kills the container; see "kill KILL [host pidns + init gone]"
		# below).
		kill -9 "$init_pid"
		wait_pids_gone 10 0.2 "$init_pid"
	fi

	# Get the list of all container processes.
	mapfile -t pids < <(cat "$cgpath"/cgroup.procs)
	echo "pids:" "${pids[@]}"
	# Sanity check -- make sure all processes exist.
	for p in "${pids[@]}"; do
		kill -0 "$p"
	done

	runc -0 kill test_busybox KILL
	# Wait and check that all processes are gone.
	wait_pids_gone 10 0.2 "${pids[@]}"

	# Make sure the container is in stopped state. Note if KILL_INIT
	# is set, container was already stopped by killing its $init_pid
	# and so this check is NOP/redundant.
	testcontainer test_busybox stopped

	# Make sure cgroup.procs is empty.
	mapfile -t pids < <(cat "$cgpath"/cgroup.procs || true)
	if [ ${#pids[@]} -gt 0 ]; then
		echo "expected empty cgroup.procs, got:" "${pids[@]}" 1>&2
		return 1
	fi
}

@test "kill detached busybox" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 kill test_busybox KILL
	wait_for_container 10 1 test_busybox stopped

	# Check that kill errors on a stopped container.
	runc ! kill test_busybox 0
	[[ "$output" == *"container not running"* ]]

	# Check that -a (now obsoleted) makes kill return no error for a stopped container.
	runc -0 kill -a test_busybox 0

	runc -0 delete test_busybox
}

# This is roughly the same as TestPIDHostInitProcessWait in libcontainer/integration.
# The differences are:
#
# 1. Here we use separate processes to create and to kill a container, so the
#    processes inside a container are not children of "runc kill".
#
# 2. We hit different codepaths (nonChildProcess.signal rather than initProcess.signal).
@test "kill KILL [host pidns]" {
	unset KILL_INIT
	test_host_pidns_kill
}

# Same as above plus:
#
# 3. Test runc kill on a container whose init process is gone.
#
# Issue 4047, case "runc kill".
# See also: "runc delete --force [host pidns + init gone]" test in delete.bats.
@test "kill KILL [host pidns + init gone]" {
	# Apparently, for rootless test, when using systemd cgroup manager,
	# newer versions of systemd clean up the container as soon as its init
	# process is gone. This is all fine and dandy, except it prevents us to
	# test this case, thus we skip the test.
	#
	# It is not entirely clear which systemd version got this feature:
	# v245 works fine, and v249 does not.
	if [ $EUID -ne 0 ] && [ -v RUNC_USE_SYSTEMD ] && [ "$(systemd_version)" -gt 245 ]; then
		skip "rootless+systemd conflicts with systemd > 245"
	fi
	KILL_INIT=1
	test_host_pidns_kill
	unset KILL_INIT
}

# https://github.com/opencontainers/runc/issues/4394 (cgroup v1, rootless)
@test "kill KILL [shared pidns]" {
	update_config '.process.args = ["sleep", "infinity"]'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" target_ctr
	testcontainer target_ctr running
	target_pid="$(__runc state target_ctr | jq .pid)"
	update_config '.linux.namespaces |= map(if .type == "user" or .type == "pid" then (.path = "/proc/'"$target_pid"'/ns/" + .type) else . end) | del(.linux.uidMappings) | del(.linux.gidMappings)'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" attached_ctr
	testcontainer attached_ctr running

	runc -0 kill attached_ctr 9

	runc -0 delete --force attached_ctr

	runc -0 delete --force target_ctr
}
