#!/bin/bash
#
# Build every commit in BASE..HEAD and check that the (stripped) runc binary
# does not grow by more than MAX_GROWTH bytes compared to the merge base.
# As a side effect, this also checks that every commit compiles.
#
# Usage: check-binary-size.sh BASE [HEAD]
#
# Environment:
#   MAX_GROWTH    maximum allowed growth, in bytes (default: 51200).
#   ALLOW_GROWTH  if set to 1, report excessive growth but do not fail.
#
# The commits are built in a temporary git worktree, so the current working
# tree (including any uncommitted or untracked files) is not used or modified.

set -Eeuo pipefail

main() {
	local max=${MAX_GROWTH:-51200}
	local base head tmp
	local -a commits

	if [ $# -lt 1 ] || [ $# -gt 2 ]; then
		echo "Usage: $0 BASE [HEAD]" >&2
		exit 1
	fi
	head=$(git rev-parse --verify "${2:-HEAD}^{commit}")
	base=$(git merge-base "$1" "$head")
	mapfile -t commits < <(git rev-list --reverse "$base..$head")
	if [ ${#commits[@]} -eq 0 ]; then
		echo "No commits in $base..$head, nothing to check."
		exit 0
	fi

	tmp=$(mktemp -d)
	# shellcheck disable=SC2064 # Expand now, not on exit.
	trap "rm -rf '$tmp'; git worktree prune" EXIT
	git worktree add -q --detach "$tmp/wt" "$base"

	local base_size size delta c d failed=0
	build "$base" "$tmp"
	base_size=$size
	d=$(desc "$base")
	echo "Base: $d: $base_size bytes"
	summary "| Commit | Size | Growth |"
	summary "|---|---:|---:|"
	summary "| base: ${d//|/\\|} | $base_size | |"

	for c in "${commits[@]}"; do
		build "$c" "$tmp"
		delta=$((size - base_size))
		d=$(desc "$c")
		echo "Commit: $d: $size bytes ($(signed "$delta"))"
		summary "| ${d//|/\\|} | $size | $(signed "$delta") |"
		if [ "$delta" -gt "$max" ]; then
			failed=1
			error "runc binary grew by $delta bytes at commit $d (max allowed is $max)"
		fi
	done

	if [ "$failed" -eq 0 ]; then
		echo "OK: runc binary growth is within $max bytes."
		return
	fi
	if [ "${ALLOW_GROWTH:-0}" = 1 ]; then
		echo "Binary growth is above the limit, but it is explicitly allowed."
		return
	fi
	echo "Binary growth is above the limit. Please investigate, and fix if possible."
	exit 1
}

# Check out a commit in the temporary worktree, build runc, and set size to
# the size of the stripped binary. Not to be called from a subshell, as errexit
# won't work there.
build() {
	local commit=$1 tmp=$2 d
	local wt=$tmp/wt

	d=$(desc "$commit")
	git -C "$wt" checkout -q "$commit"
	echo "Building $d"
	rm -f "$wt/runc"
	if ! make -C "$wt" runc; then
		error "failed to build commit $d"
		exit 1
	fi
	strip -o "$tmp/runc" "$wt/runc"
	size=$(stat -c %s "$tmp/runc")
}

desc() {
	git log -1 --no-show-signature --format='%h %s' "$1"
}

signed() {
	printf "%+d" "$1"
}

summary() {
	if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
		echo "$*" >>"$GITHUB_STEP_SUMMARY"
	fi
}

error() {
	if [ -n "${GITHUB_ACTIONS:-}" ]; then
		echo "::error::$*"
	else
		echo "ERROR: $*" >&2
	fi
}

main "$@"
exit
