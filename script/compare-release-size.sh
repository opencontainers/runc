#!/bin/bash
#
# Compare the sizes of the release binaries in DIR (as built by "make
# releaseall") with those of the base commit. The base binaries are taken
# from the release artifact which the validate workflow uploads on every push
# to BRANCH: the one for SHA if available, otherwise the latest one.
#
# Usage: compare-release-size.sh BRANCH SHA DIR
#
# Requires gh, authenticated (e.g. via GH_TOKEN) with actions:read permission.
# The results are printed, and also added to $GITHUB_STEP_SUMMARY (if set).
#
# This is purely informational: size changes, or a missing base artifact,
# do not result in an error.

set -Eeuo pipefail

repo=${GITHUB_REPOSITORY:-opencontainers/runc}

main() {
	if [ $# -ne 3 ]; then
		echo "Usage: $0 BRANCH SHA DIR" >&2
		exit 1
	fi
	local branch=$1 sha=$2 dir=$3
	local tmp run_id run_sha note=""

	tmp=$(mktemp -d)
	# shellcheck disable=SC2064 # Expand now, not on exit.
	trap "rm -rf '$tmp'" EXIT

	# Try the runs for the base commit first, then the latest ones for
	# the branch. A run may have no artifact (e.g. if it is still in
	# progress, or the release job has failed, or it has expired).
	while read -r run_id run_sha; do
		if gh run download "$run_id" --repo "$repo" \
			--name "release-$run_id" --dir "$tmp/base" 2>/dev/null; then
			break
		fi
		run_id=""
	done < <(
		list_runs "$branch" --commit "$sha"
		list_runs "$branch"
	)

	if [ -z "$run_id" ]; then
		note="No release artifact found for $branch, nothing to compare with."
	elif [ "$run_sha" != "$sha" ]; then
		note="No release artifact found for ${sha:0:8}, comparing with ${run_sha:0:8} (latest $branch)."
	fi

	{
		echo "### Release binaries size"
		echo
		if [ -n "$run_id" ]; then
			echo "Base: ${run_sha:0:8} (run $run_id)."
			echo
		fi
		if [ -n "$note" ]; then
			echo "$note"
			echo
		fi
		echo "| Binary | Base | This PR | Change |"
		echo "|---|---:|---:|---:|"
		join -a1 -a2 -e - -o 0,1.2,2.2 \
			<(sizes "$tmp/base") <(sizes "$dir") |
			awk '{
				d = "";
				if ($2 != "-" && $3 != "-") {
					d = sprintf("%+d (%+.1f%%)", $3 - $2, ($3 - $2) * 100 / $2);
				}
				printf "| %s | %s | %s | %s |\n", $1, $2, $3, d;
			}'
	} | tee -a "${GITHUB_STEP_SUMMARY:-/dev/null}"
}

# List IDs and commits of the latest validate workflow runs for pushes to a
# branch (additional gh run list arguments can be specified).
list_runs() {
	local branch=$1
	shift
	gh run list --repo "$repo" --workflow validate.yml \
		--event push --branch "$branch" --limit 5 "$@" \
		--json databaseId,headSha --jq '.[] | "\(.databaseId) \(.headSha)"'
}

# Print the names and sizes of runc release binaries found in a directory
# (or its subdirectories), sorted by name.
sizes() {
	[ -d "$1" ] || return 0
	find "$1" -type f -name 'runc.*' \
		! -name '*.asc' ! -name 'runc.sha256sum' \
		-printf '%f %s\n' | sort
}

main "$@"
