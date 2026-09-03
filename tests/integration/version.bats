#!/usr/bin/env bats

load helpers

@test "runc version" {
	run -0 runc -v
	assert_line --index 0 --regexp 'runc version [0-9]+\.[0-9]+\.[0-9]+'
	assert_line --index 1 --regexp 'commit:+'
	assert_line --index 2 --regexp 'spec: [0-9]+\.[0-9]+\.[0-9]+'
}
