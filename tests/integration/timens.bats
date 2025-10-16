#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run [timens offsets with no timens]" {
	requires timens

	update_config '.process.args = ["cat", "/proc/self/timens_offsets"]'
	update_config '.linux.namespaces = .linux.namespace | map(select(.type != "time"))'
	update_config '.linux.timeOffsets = {
			"monotonic": { "secs": 7881, "nanosecs": 2718281 },
			"boottime": { "secs": 1337, "nanosecs": 3141519 }
		}'

	runc ! run test_busybox
}

@test "runc run [timens with no offsets]" {
	requires timens

	update_config '.process.args = ["cat", "/proc/self/timens_offsets"]'
	update_config '.linux.namespaces += [{"type": "time"}]
		| .linux.timeOffsets = null'

	runc -0 run test_busybox
	# Default offsets are 0.
	grep -E '^monotonic\s+0\s+0$' <<<"$output"
	grep -E '^boottime\s+0\s+0$' <<<"$output"
}

@test "runc run [simple timens]" {
	requires timens

	update_config '.process.args = ["cat", "/proc/self/timens_offsets"]'
	update_config '.linux.namespaces += [{"type": "time"}]
		| .linux.timeOffsets = {
			"monotonic": { "secs": 7881, "nanosecs": 2718281 },
			"boottime": { "secs": 1337, "nanosecs": 3141519 }
		}'

	runc -0 run test_busybox
	grep -E '^monotonic\s+7881\s+2718281$' <<<"$output"
	grep -E '^boottime\s+1337\s+3141519$' <<<"$output"
}

# https://github.com/opencontainers/runc/issues/4635
@test "runc exec [simple timens]" {
	requires timens

	update_config '.process.args = ["sleep", "inf"]'
	update_config '.linux.namespaces += [{"type": "time"}]
		| .linux.timeOffsets = {
			"monotonic": { "secs": 7881, "nanosecs": 2718281 },
			"boottime": { "secs": 1337, "nanosecs": 3141519 }
		}'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	runc -0 exec test_busybox cat /proc/self/timens_offsets
	grep -E '^monotonic\s+7881\s+2718281$' <<<"$output"
	grep -E '^boottime\s+1337\s+3141519$' <<<"$output"
}

@test "runc run [simple timens + userns]" {
	requires root
	requires timens

	update_config ' .linux.namespaces += [{"type": "user"}]
		| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
		| .linux.gidMappings += [{"hostID": 200000, "containerID": 0, "size": 65534}] '
	remap_rootfs

	update_config '.process.args = ["cat", "/proc/self/timens_offsets"]'
	update_config '.linux.namespaces += [{"type": "time"}]
		| .linux.timeOffsets = {
			"monotonic": { "secs": 7881, "nanosecs": 2718281 },
			"boottime": { "secs": 1337, "nanosecs": 3141519 }
		}'

	runc -0 run test_busybox
	grep -E '^monotonic\s+7881\s+2718281$' <<<"$output"
	grep -E '^boottime\s+1337\s+3141519$' <<<"$output"
}
