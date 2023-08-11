#!/usr/bin/env bats

load helpers

function setup() {
	requires root no_systemd

	setup_debian
	# CR = CreateRuntime, CC = CreateContainer
	HOOKLIBCR=librunc-hooks-create-runtime.so
	HOOKLIBCC=librunc-hooks-create-container.so
	LIBPATH="$(pwd)/rootfs/lib/"
}

function teardown() {
	if [ -v LIBPATH ]; then
		umount "$LIBPATH/$HOOKLIBCR".1.0.0 &>/dev/null || true
		umount "$LIBPATH/$HOOKLIBCC".1.0.0 &>/dev/null || true
		rm -f "$HOOKLIBCR".1.0.0 "$HOOKLIBCC".1.0.0
		unset LIBPATH HOOKLIBCR HOOKLIBCC
	fi
	teardown_bundle
}

@test "runc run (hooks library tests)" {
	# setup some dummy libs
	gcc -shared -Wl,-soname,librunc-hooks-create-runtime.so.1 -o "$HOOKLIBCR.1.0.0"
	gcc -shared -Wl,-soname,librunc-hooks-create-container.so.1 -o "$HOOKLIBCC.1.0.0"

	bundle=$(pwd)

	# To mount $HOOKLIBCR we need to do that in the container namespace
	create_runtime_hook=$(
		cat <<-EOF
			pid=\$(cat - | jq -r '.pid')
			touch "$LIBPATH/$HOOKLIBCR.1.0.0"
			nsenter -m \$ns -t \$pid mount --bind "$bundle/$HOOKLIBCR.1.0.0" "$LIBPATH/$HOOKLIBCR.1.0.0"
		EOF
	)

	create_container_hook="touch ./lib/$HOOKLIBCC.1.0.0 && mount --bind $bundle/$HOOKLIBCC.1.0.0 ./lib/$HOOKLIBCC.1.0.0"

	# shellcheck disable=SC2016
	update_config --arg create_runtime_hook "$create_runtime_hook" --arg create_container_hook "$create_container_hook" '
		.hooks |= . + {"createRuntime": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", $create_runtime_hook]}]} |
		.hooks |= . + {"createContainer": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", $create_container_hook]}]} |
		.hooks |= . + {"startContainer": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", "ldconfig"]}]} |
		.root.readonly |= false |
		.process.args = ["/bin/sh", "-c", "ldconfig -p | grep librunc"]'

	runc run test_debian
	[ "$status" -eq 0 ]

	echo "Checking create-runtime library"
	echo "$output" | grep "$HOOKLIBCR"

	echo "Checking create-container library"
	echo "$output" | grep "$HOOKLIBCC"
}
