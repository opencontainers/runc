#!/usr/bin/env bats

load helpers

function setup() {
	# It does not make sense to repeat these trivial tests for non-root.
	requires root
}

# A simplified replacement for the function from bash-completion package,
# sufficient for the test cases below.
# shellcheck disable=SC2034 # Variables are used by the caller.
function _get_comp_words_by_ref() {
	cur=${COMP_WORDS[COMP_CWORD]}
	prev=${COMP_WORDS[COMP_CWORD - 1]}
	words=("${COMP_WORDS[@]}")
	cword=$COMP_CWORD
}

# Print sorted bash completion results for "runc $@", with the last
# argument being the word to complete.
function complete_runc() {
	# The completion code is not meant to be run with errexit.
	set +e
	# shellcheck disable=SC1091
	source "${INTEGRATION_ROOT}/../../contrib/completions/bash/runc"

	local COMP_WORDS=(runc "$@")
	local COMP_CWORD=$((${#COMP_WORDS[@]} - 1))
	local COMPREPLY=()
	_runc

	printf '%s\n' "${COMPREPLY[@]}" | sort -u
}

# Print sorted options from a given section of "runc $@ --help" output.
function help_options() {
	local section=$1
	shift

	"$RUNC" "$@" --help | awk -v section="$section" '
		$0 == section { p = 1; next }
		/^[^ ]/ { p = 0 }
		p && /^   -/ {
			sub(/^ +/, "")
			sub(/  .*/, "")  # Remove the description.
			sub(/ \[.*/, "") # Remove [ ... ] from repeatable options.
			n = split($0, opts, ", ")
			for (i = 1; i <= n; i++) {
				split(opts[i], o, " ")
				print o[1]
			}
		}' | sort -u
}

# Print commands from "runc --help" output, except "features" which is
# meant to be used by higher level runtimes, not interactively.
function help_commands() {
	"$RUNC" --help | awk '
		$0 == "COMMANDS:" { p = 1; next }
		p && /^$/ { exit }
		p && $1 != "features" { sub(/,$/, "", $1); print $1 }'
}

@test "bash completion: commands" {
	diff -u <(help_commands | sort -u) <(complete_runc "")
}

@test "bash completion: global options" {
	diff -u <(help_options "GLOBAL OPTIONS:") <(complete_runc -)
}

@test "bash completion: command options" {
	local cmd failed=0
	for cmd in $(help_commands); do
		[ "$cmd" = "help" ] && continue
		echo "== runc $cmd"
		diff -u <(help_options "OPTIONS:" "$cmd") <(complete_runc "$cmd" -) || failed=1
	done
	[ "$failed" -eq 0 ]
}
