#!/bin/bash
#
# The assert_* functions below implement a subset of the bats-assert library
# (https://github.com/bats-core/bats-assert). We do not use the real thing
# because it, together with its bats-support dependency, has to be installed
# separately on every distribution the tests are run on.
#
#	assert_output [--partial | --regexp] EXPECTED
#	refute_output [--partial | --regexp] UNEXPECTED
#	assert_line --index IDX [--partial | --regexp] EXPECTED
#
# This file lives in its own directory so that bats can be told to skip it
# when capturing stack traces (see below); otherwise every failed assertion
# is prefixed by a few useless frames pointing into this file.
if declare -F bats_setup_tracing >/dev/null &&
	! declare -F __assert_orig_setup_tracing >/dev/null; then
	# Rename bats_setup_tracing to __assert_orig_setup_tracing ...
	eval "__assert_orig_setup_tracing() $(declare -f bats_setup_tracing | tail -n +2)"
	# ... and wrap it, so that this directory is added to the list of paths
	# excluded from stack traces. This can not be done at load time, as bats
	# resets the exclude list in bats_setup_tracing, which runs afterwards.
	function bats_setup_tracing() {
		__assert_orig_setup_tracing "$@"
		bats_add_debug_exclude_path "${BASH_SOURCE[0]%/*}"
	}
fi

# __assert_matches MODE ACTUAL EXPECTED
function __assert_matches() {
	case "$1" in
	exact) [ "$2" = "$3" ] ;;
	partial) [[ $2 == *"$3"* ]] ;;
	regexp) [[ $2 =~ $3 ]] ;;
	*) fail "assert: unknown mode: $1" ;;
	esac
}

# __assert_fail WHAT MODE EXPECTED ACTUAL
function __assert_fail() {
	{
		echo "-- $1 ($2) --"
		echo "expected: ${3@Q}"
		echo "  actual: ${4@Q}"
		echo "--"
	} >&2

	return 1
}

# __assert_mode ARG -- prints the match mode for an optional flag.
function __assert_mode() {
	case "$1" in
	--partial) echo partial ;;
	--regexp) echo regexp ;;
	# Guard against a typo or an unimplemented bats-assert option being
	# silently treated as the expected value.
	-*) fail "assert: unknown option: $1" ;;
	*) echo exact ;;
	esac
}

function assert_output() {
	local mode
	mode=$(__assert_mode "${1:-}")
	[ "$mode" = exact ] || shift

	# shellcheck disable=SC2154 # $output is set by bats' run helper.
	__assert_matches "$mode" "$output" "$1" && return 0
	__assert_fail "output does not match" "$mode" "$1" "$output"
}

function refute_output() {
	local mode
	mode=$(__assert_mode "${1:-}")
	[ "$mode" = exact ] || shift

	# shellcheck disable=SC2154 # $output is set by bats' run helper.
	__assert_matches "$mode" "$output" "$1" || return 0
	__assert_fail "output should not match" "$mode" "$1" "$output"
}

function assert_line() {
	local idx mode
	[ "${1:-}" = "--index" ] || fail "assert_line: --index IDX is required"
	idx="$2"
	shift 2
	mode=$(__assert_mode "${1:-}")
	[ "$mode" = exact ] || shift

	# shellcheck disable=SC2154 # $lines is set by bats' run helper.
	__assert_matches "$mode" "${lines[idx]:-}" "$1" && return 0
	__assert_fail "line $idx does not match" "$mode" "$1" "${lines[idx]:-}"
}
