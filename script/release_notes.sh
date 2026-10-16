#!/bin/bash
# Generate the text for a runc release: either the annotated tag message
# (-m), or the GitHub release title (-t) or notes (default).
#
# The release process is:
#
# 1. With the release branch checked out, generate the tag message template:
#
#	./script/release_notes.sh -m vX.Y.Z > tag-msg
#
#    It contains the release title (with the quote from CHANGELOG.md), a
#    preamble placeholder, the CHANGELOG.md section in plain text, and the
#    list of contributors (generated from git history since the previous
#    release). The release commit (the latest one setting VERSION to X.Y.Z)
#    is found automatically, and printed.
#
# 2. Edit tag-msg (write the preamble, review the rest), then create and
#    push a signed annotated tag for the release commit:
#
#	git tag -s -F tag-msg vX.Y.Z <release-commit>
#
# 3. CI then creates a draft GitHub release with the title, the preamble,
#    and the list of contributors taken from the tag message, the
#    CHANGELOG.md section (in Markdown), and the static linking notices.

set -Eeuo pipefail

root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

function usage() {
	echo "usage: release_notes.sh [-m [-p <prev-tag>] | -t] <version>" >&2
	echo "  -m  print the tag message template" >&2
	echo "  -p  previous release tag (default: the closest tag before the release)" >&2
	echo "  -t  print the release title (from the tag message)" >&2
	exit 1
}

mode=notes
prev=
while getopts "hmp:t" opt; do
	case "$opt" in
	m)
		mode=tag
		;;
	p)
		prev="$OPTARG"
		;;
	t)
		mode=title
		;;
	*)
		usage
		;;
	esac
done
shift $((OPTIND - 1))
[ $# -eq 1 ] || usage
# Allow the version to be specified as a tag (with a leading v).
version="${1#v}"
tag="v$version"

# Use the release tag if it exists. Otherwise (i.e. when generating the
# message for the tag to be created), use the release commit, which is the
# latest commit (reachable from HEAD) setting VERSION to $version.
if git -C "$root" rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
	ref="$tag"
elif [ "$mode" != tag ]; then
	echo "tag $tag not found" >&2
	exit 1
else
	ref=
	for c in $(git -C "$root" log --format=%h HEAD -- VERSION); do
		if [ "$(git -C "$root" show "$c:VERSION")" = "$version" ]; then
			ref="$c"
			break
		fi
	done
	if [ -z "$ref" ]; then
		echo "no commit setting VERSION to $version found (in HEAD history)" >&2
		exit 1
	fi
	echo "release commit: $(git -C "$root" log -1 --format='%h ("%s")' "$ref")" >&2
fi
changelog="$(git -C "$root" show "$ref:CHANGELOG.md")"

# Print the CHANGELOG.md section for $version (without the header).
function section() {
	awk -v hdr="## [$version]" '
		index($0, hdr) == 1 { found = 1; next }
		found && /^## \[/ { exit }
		found { print }
	' <<<"$changelog"
}

# Print the static linking notices for the release binaries.
function static_linking_notices() {
	cat <<'EOF'
### Static Linking Notices ###

The `runc` binaries distributed with this release are *statically linked* with
the following [GNU LGPL-2.1][lgpl-2.1] licensed libraries, with `runc` acting
as a "work that uses the Library":

[lgpl-2.1]: https://www.gnu.org/licenses/old-licenses/lgpl-2.1.en.html

 - [libseccomp](https://github.com/seccomp/libseccomp)
EOF
	if grep -q LIBPATHRS_VERSION <<<"$(git -C "$root" show "$ref:script/release_build.sh")"; then
		cat <<'EOF'

Similarly, the `runc` binaries distributed with this release are also
*statically linked* with the following [MPLv2][mpl-2.0] licensed libraries,
with `runc` acting as a "Larger Work":

 - [libpathrs](https://github.com/cyphar/libpathrs)

[mpl-2.0]: https://www.mozilla.org/en-US/MPL/2.0/
EOF
	fi
	cat <<'EOF'

The versions of these libraries were not modified from their upstream versions,
but in order to comply with their corresponding licenses, we have attached the
complete source code for those libraries which (when combined with the attached
runc source code) may be used to exercise your rights under their respective
licenses.

However, we strongly suggest that you make use of your distribution's packages
or download them from the authoritative upstream sources, especially since
these libraries are related to the security of your containers.
EOF
}

# Print the list of contributors: commit authors, people mentioned in commit
# trailers (Co-authored-by, Reported-by, etc.), and maintainers who approved
# merges (LGTMs in merge commits).
function contributors() {
	local range="$prev..$ref"
	local maintainers="$root/MAINTAINERS"

	{
		git -C "$root" log --no-merges --use-mailmap --format='%aN <%aE>' "$range"
		git -C "$root" log --format='%(trailers:only,unfold,separator=%x0a)' "$range" |
			sed -nE 's/^[A-Za-z-]+: *(.* <[^>]+@[^>]+>)$/\1/p'
		git -C "$root" log --merges --format='%(trailers:key=LGTMs,valueonly,separator=%x20)' "$range" |
			tr -s ' ' '\n' | sort -u | while read -r login; do
			[ -z "$login" ] && continue
			if ! grep -F "(@$login)" "$maintainers"; then
				echo "warning: $login not found in MAINTAINERS" >&2
			fi
		done
	} | grep -v -e '\[bot\]' -e '<noreply@anthropic.com>' |
		# Use the names from MAINTAINERS, and remove duplicates (entries
		# having the same name or the same email as an earlier entry).
		awk -v maintainers="$maintainers" '
			function email(s) { return match(s, /<[^>]+>/) ? tolower(substr(s, RSTART, RLENGTH)) : "" }
			function name(s) { s = tolower(s); sub(/ *<.*/, "", s); return s }
			BEGIN {
				while ((getline l < maintainers) > 0) {
					sub(/ *\(@.*/, "", l)
					canon[email(l)] = l
				}
			}
			{
				e = email($0)
				n0 = name($0)
				if (e in canon) $0 = canon[e]
				n = name($0)
				if ((n0 in names) || (n in names) || (e in emails)) next
				names[n0]; names[n]; emails[e]
				print " * " $0
			}' |
		sort -f
}

# Convert Markdown to plain text, for the tag message.
function plain() {
	# Replace shortcut reference links ([label], having a "[label]: url"
	# definition) with their labels.
	awk '
		/^\[[^]]+\]: / { l = $0; sub(/^\[/, "", l); sub(/\]:.*/, "", l); labels[l] }
		{ lines[NR] = $0 }
		END {
			for (i = 1; i <= NR; i++) {
				s = lines[i]
				for (l in labels) {
					pat = "[" l "]"
					out = ""
					while ((p = index(s, pat)) > 0) {
						end = p + length(pat)
						# Keep [label][...], [label](...), and [label]: definitions.
						if (substr(s, end, 1) ~ /[[(:]/) out = out substr(s, 1, end - 1)
						else out = out substr(s, 1, p - 1) l
						s = substr(s, end)
					}
					s = out s
				}
				print s
			}
		}
	' | sed -E \
		-e '/^\[[^]]+\]: /d' \
		-e 's/^### (.*) ###$/\1:\n/' \
		-e 's/\[([^]]+)\](\[[^]]*\]|\([^)]*\))/\1/g' |
		cat -s
}

# Remove leading and trailing empty lines.
function trim() {
	sed -e '/[^[:space:]]/,$!d' | tac | sed -e '/[^[:space:]]/,$!d' | tac
}

notes="$(section)"
if ! grep -q '[^[:space:]]' <<<"$notes"; then
	echo "no '## [$version]' section found in CHANGELOG.md ($ref)" >&2
	exit 1
fi

# A section starts with an optional quote (a Markdown blockquote), which
# becomes a part of the release title. The rest are release notes.
quote="$(awk '
	!started && /^[[:space:]]*$/ { next }
	/^>/ { started = 1; sub(/^> ?/, ""); printf "%s%s", sep, $0; sep = " "; next }
	{ exit }
' <<<"$notes")"
notes="$(awk '
	!started && (/^[[:space:]]*$/ || /^>/) { next }
	{ started = 1; print }
' <<<"$notes")"

# Add link reference definitions which are used in the section but
# defined elsewhere in CHANGELOG.md.
defs="$(awk -v notes="$notes" '
	function label(s) { sub(/^\[/, "", s); sub(/\]:.*/, "", s); return tolower(s) }
	BEGIN {
		n = split(notes, l, "\n")
		for (i = 1; i <= n; i++) {
			if (l[i] ~ /^\[[^]]+\]: /) defined[label(l[i])]
			else text = text "\n" tolower(l[i])
		}
	}
	/^\[[^]]+\]: / {
		lb = label($0)
		if (!(lb in defined) && index(text, "[" lb "]")) { print; defined[lb] }
	}
' <<<"$changelog")"
if [ -n "$defs" ]; then
	notes+=$'\n\n'"$defs"
fi

if [ "$mode" = tag ]; then
	if [ -z "$prev" ]; then
		prev="$(git -C "$root" describe --tags --abbrev=0 --match 'v[0-9]*' --exclude "$tag" "$ref")"
	else
		prev="v${prev#v}"
		if ! git -C "$root" rev-parse -q --verify "refs/tags/$prev" >/dev/null; then
			echo "previous release tag $prev not found" >&2
			exit 1
		fi
	fi
	echo "generating the list of contributors for $prev..$ref" >&2
	thanks="$(contributors)"
	cat <<EOF
runc $tag${quote:+ -- \"$quote\"}

TODO: write a preamble here.

$(plain <<<"$notes" | trim)

Thanks to the following contributors who made this release possible:

$thanks

Signed-off-by: $(git -C "$root" var GIT_COMMITTER_IDENT | sed -E 's/ [0-9]+ [-+][0-9]+$//')
EOF
	if [ "$ref" != "$tag" ]; then
		echo "after editing the message, create the tag with: git tag -s -F <file> $tag $ref" >&2
	fi
	exit 0
fi

# The title, the preamble, and the list of contributors are taken
# from the annotated tag message.
if [ "$(git -C "$root" cat-file -t "$tag")" != tag ]; then
	echo "$tag is not an annotated tag" >&2
	exit 1
fi
if [ "$mode" = title ]; then
	git -C "$root" for-each-ref "refs/tags/$tag" --format='%(contents:subject)'
	exit 0
fi
msg="$(git -C "$root" for-each-ref "refs/tags/$tag" --format='%(contents:body)')"

# The preamble is everything before the first CHANGELOG.md subsection.
first="$(sed -nE 's/^### (.*) ###$/\1:/p' <<<"$notes" | head -n1)"
if ! grep -qxF "$first" <<<"$msg"; then
	echo "$tag message: can't find the preamble end ($first)" >&2
	exit 1
fi
preamble="$(awk -v end="$first" '$0 == end { exit } { print }' <<<"$msg" | trim)"

thanks_hdr='Thanks to the following contributors'
if ! grep -q "^$thanks_hdr" <<<"$msg"; then
	echo "$tag message: can't find '$thanks_hdr'" >&2
	exit 1
fi
thanks="$(sed -n "/^$thanks_hdr/,\$p" <<<"$msg" | trim)"

cat <<EOF
$preamble

$notes

$(static_linking_notices)

- - -

$thanks
EOF
