#!/usr/bin/env bats

load helpers

function setup() {
	requires root
}

function _get_comp_words_by_ref() {
	local OPTIND _opt
	while getopts "n:" _opt; do
		:
	done
	shift $((OPTIND - 1))

	local cur_var=$1
	local prev_var=$2
	local words_var=$3
	local cword_var=$4

	printf -v "$cur_var" '%s' "${COMP_WORDS[COMP_CWORD]}"
	if ((COMP_CWORD > 0)); then
		printf -v "$prev_var" '%s' "${COMP_WORDS[COMP_CWORD - 1]}"
	else
		printf -v "$prev_var" ''
	fi
	eval "$words_var=()"
	local i
	for ((i = 0; i < ${#COMP_WORDS[@]}; i++)); do
		eval "$words_var+=(\"\${COMP_WORDS[$i]}\")"
	done
	printf -v "$cword_var" '%s' "$COMP_CWORD"
}

function _filedir() {
	:
}

function compopt() {
	return 0
}

function complete_runc() {
	# shellcheck disable=SC1091
	source "${INTEGRATION_ROOT}/../../contrib/completions/bash/runc"

	COMP_WORDS=("$@")
	COMP_CWORD=$((${#COMP_WORDS[@]} - 1))
	COMPREPLY=()
	_runc

	printf '%s\n' "${COMPREPLY[@]}"
}

function contains_reply() {
	local needle=$1
	shift

	local reply
	for reply in "$@"; do
		if [ "$reply" = "$needle" ]; then
			return 0
		fi
	done

	return 1
}

@test "top-level completion includes features" {
	mapfile -t replies < <(complete_runc runc fe)

	contains_reply "features" "${replies[@]}"
}

@test "delete completion suggests --force without a trailing comma" {
	mapfile -t replies < <(complete_runc runc delete --f)

	contains_reply "--force" "${replies[@]}"
	run ! contains_reply "--force," "${replies[@]}"
}

@test "run completion matches supported flags" {
	mapfile -t replies < <(complete_runc runc run --k)
	contains_reply "--keep" "${replies[@]}"

	mapfile -t replies < <(complete_runc runc run --d)
	contains_reply "--detach" "${replies[@]}"
	run ! contains_reply "--detatch" "${replies[@]}"

	mapfile -t replies < <(complete_runc runc run --pidf)
	contains_reply "--pidfd-socket" "${replies[@]}"
}

@test "exec completion includes pidfd socket and plain long flags" {
	mapfile -t replies < <(complete_runc runc exec --pidf)
	contains_reply "--pidfd-socket" "${replies[@]}"

	mapfile -t replies < <(complete_runc runc exec --t)
	contains_reply "--tty" "${replies[@]}"
	run ! contains_reply "--tty," "${replies[@]}"
}

@test "format completions use table or json" {
	mapfile -t replies < <(complete_runc runc list --format "")
	contains_reply "table" "${replies[@]}"
	contains_reply "json" "${replies[@]}"
	run ! contains_reply "text" "${replies[@]}"

	mapfile -t replies < <(complete_runc runc ps --format "")
	contains_reply "table" "${replies[@]}"
	contains_reply "json" "${replies[@]}"
}
