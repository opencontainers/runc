#!/usr/bin/env bats

load helpers

@test "runc version" {
	run -0 runc -v
	[[ ${lines[0]} =~ runc\ version\ [0-9]+\.[0-9]+\.[0-9]+ ]]
	[[ ${lines[1]} =~ commit:+ ]]
	[[ ${lines[2]} =~ spec:\ [0-9]+\.[0-9]+\.[0-9]+ ]]
}
