#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run memory policy interleave without flags" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "MPOL_INTERLEAVE",
		"nodes": "0"
	}'
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "interleave:0" ]]
}

@test "runc run memory policy bind static" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "MPOL_BIND",
		"nodes": "0",
		"flags": ["MPOL_F_STATIC_NODES"]
	}'
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "bind"*"static"*"0" ]]
}

@test "runc run and exec memory policy prefer relative" {
	update_config '
	.linux.memoryPolicy = {
		"mode": "MPOL_PREFERRED",
		"nodes": "0",
		"flags": ["MPOL_F_RELATIVE_NODES"]
	}'
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox /bin/sh -c "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "prefer"*"relative"*"0" ]]
}

@test "runc run empty memory policy" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
	}'
	runc run test_busybox
	[ "$status" -eq 1 ]
	[[ "${lines[0]}" == *"invalid memory policy"* ]]
}

@test "runc run memory policy with non-existing mode" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "INTERLEAVE",
		"nodes": "0"
	}'
	runc run test_busybox
	[ "$status" -eq 1 ]
	[[ "${lines[0]}" == *"invalid memory policy"* ]]
}

@test "runc run memory policy with invalid flag" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "MPOL_PREFERRED",
		"nodes": "0",
		"flags": ["MPOL_F_RELATIVE_NODES", "badflag"]
	}'
	runc run test_busybox
	[ "$status" -eq 1 ]
	[[ "${lines[0]}" == *"invalid memory policy flag"* ]]
}

@test "runc run memory policy default with missing nodes" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "MPOL_DEFAULT"
	}'
	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *"default"* ]]
}

@test "runc run memory policy with missing mode" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"nodes": "0-7"
	}'
	runc run test_busybox
	[ "$status" -eq 1 ]
	[[ "${lines[0]}" == *"invalid memory policy mode"* ]]
}

@test "runc run memory policy calls syscall with invalid arguments" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "MPOL_DEFAULT",
		"nodes": "0-7",
	}'
	runc run test_busybox
	[ "$status" -eq 1 ]
	[[ "${lines[*]}" == *"mode requires 0 nodes but got 8"* ]]
}

@test "runc run memory policy bind way too large a node number" {
	update_config '
	.process.args = ["/bin/sh", "-c", "head -n 1 /proc/self/numa_maps | cut -d \" \" -f 2"]
	| .linux.memoryPolicy = {
		"mode": "MPOL_BIND",
		"nodes": "0-9876543210",
		"flags": []
	}'
	runc run test_busybox
	[ "$status" -eq 1 ]
	[[ "${lines[0]}" == *"invalid memory policy node"* ]]
}
