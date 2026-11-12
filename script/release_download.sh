#!/bin/bash
# Download the artifacts of a draft GitHub release (created by CI on a tag
# push) so they can be signed locally before publishing the release.

set -Eeuo pipefail

project="runc"
root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

function usage() {
	echo "usage: release_download.sh [-r <release-dir>] [-v <version>] [-R <owner/repo>]" >&2
	exit 1
}

version=""
releasedir=""
repo=""

while getopts "hr:R:v:" opt; do
	case "$opt" in
	r)
		releasedir="$OPTARG"
		;;
	R)
		repo="$OPTARG"
		;;
	v)
		version="$OPTARG"
		;;
	*)
		usage
		;;
	esac
done

version="${version:-$(<"$root/VERSION")}"
# Allow the version to be specified as a tag (with a leading v).
version="${version#v}"
releasedir="${releasedir:-release/$version}"
repo="${repo:-opencontainers/$project}"
tag="v$version"

if [ -e "$releasedir" ]; then
	echo "$releasedir already exists, remove it first" >&2
	exit 1
fi

set -x
gh release download "$tag" --repo "$repo" --dir "$releasedir"
url="$(gh release view "$tag" --repo "$repo" --json url --jq .url)"
set +x

cat >&2 <<EOF

Downloaded the $repo $tag release into $releasedir. Now:

1. Sign the artifacts:

	./script/release_sign.sh -S <gpg-key-id> -r $releasedir -v $tag

2. Upload the signatures and the signed checksums file:

	gh release upload $tag --repo $repo --clobber \\
		$releasedir/*.asc $releasedir/$project.sha256sum

3. Go to $url
   and check everything (title, release notes, artifacts, signatures).
   Click "Edit", check the release notes and the "Set as the latest
   release" checkbox. If all is good, click "Publish release".
EOF
