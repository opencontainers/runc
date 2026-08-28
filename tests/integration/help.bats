#!/usr/bin/env bats

load helpers

function setup() {
	# It does not make sense to repeat these trivial tests for non-root.
	# Also, they fail due to $ROOT not being set and XDG_RUNTIME_DIR
	# pointing to another user's directory after sudo rootless.
	requires root
}

@test "runc -h" {
	run -0 runc -h
	[[ ${lines[0]} =~ NAME:+ ]]
	[[ ${lines[1]} =~ runc\ '-'\ Open\ Container\ Initiative\ runtime+ ]]

	run -0 runc --help
	[[ ${lines[0]} =~ NAME:+ ]]
	[[ ${lines[1]} =~ runc\ '-'\ Open\ Container\ Initiative\ runtime+ ]]
}

@test "runc command -h" {
	local bin
	bin=$(basename "$RUNC")
	local cmds=(
		checkpoint
		create
		delete
		events
		exec
		kill
		list
		pause
		ps
		restore
		resume
		run
		spec
		start
		state
		update
		features
	)

	for cmd in "${cmds[@]}"; do
		for arg in "-h" "--help"; do
			run -0 runc "$cmd" "$arg"
			[[ ${lines[0]} =~ NAME:+ ]]
			[[ ${lines[1]} =~ $bin\ $cmd+ ]]
		done
	done
}

@test "runc foo -h" {
	run ! runc foo -h
	[[ "${output}" == *"No help topic for 'foo'"* ]]
}
