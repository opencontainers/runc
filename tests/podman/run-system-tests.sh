#!/bin/bash
# Run a subset of podman system tests (test/system/*.bats) using runc
# as the OCI runtime. The list of tests is the same as crun uses.
#
# Usage: run-system-tests.sh <podman-source-dir>
#
# Podman binaries are expected to be already built (make binaries)
# in <podman-source-dir>. Podman must be configured to use runc as
# the default runtime. Can be run as root or as a rootless user.

set -Eeuo pipefail

if [ $# -ne 1 ]; then
	echo "Usage: $0 <podman-source-dir>" >&2
	exit 1
fi

PODMAN_SRC="$(realpath "$1")"
PODMAN="$PODMAN_SRC/bin/podman"
export PODMAN

TESTS=(
	030-run.bats
	060-mount.bats
	075-exec.bats
	170-run-userns.bats
	200-pod.bats
	280-update.bats
	400-unprivileged-access.bats
	420-cgroups.bats
	520-checkpoint.bats
)

set -x
runtime=$("$PODMAN" info --format '{{.Host.OCIRuntime.Path}}: {{.Host.OCIRuntime.Version}}')
echo "$runtime"
if [[ "$runtime" != */runc:* ]]; then
	echo "$0: podman default runtime is not runc" >&2
	exit 1
fi

# Tests to skip (each entry is a part of the test name, regexp).
SKIP_TESTS=(
	# Leaks a pod, making teardown_suite fail.
	# TODO: remove once https://github.com/podman-container-tools/podman/pull/28151
	# is in the podman version used.
	'podman pod inspect ordering'
)
SKIP_REGEX=$(
	IFS='|'
	echo "${SKIP_TESTS[*]}"
)

cd "$PODMAN_SRC/test/system"
bats -T --negative-filter "$SKIP_REGEX" "${TESTS[@]}"
