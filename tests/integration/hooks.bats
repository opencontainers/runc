#!/usr/bin/env bats

load helpers

# CR = CreateRuntime
# CC = CreataContainer
HOOKLIBCR=librunc-hooks-create-runtime.so
HOOKLIBCC=librunc-hooks-create-container.so
LIBPATH="$DEBIAN_BUNDLE/rootfs/lib/"

function setup() {
	umount $LIBPATH/$HOOKLIBCR.1.0.0 &> /dev/null || true
	umount $LIBPATH/$HOOKLIBCC.1.0.0 &> /dev/null || true

	teardown_debian
	setup_debian
}

function teardown() {
	umount $LIBPATH/$HOOKLIBCR.1.0.0 &> /dev/null || true
	umount $LIBPATH/$HOOKLIBCC.1.0.0 &> /dev/null || true

	rm -f $HOOKLIBCR.1.0.0 $HOOKLIBCC.1.0.0
	teardown_debian
}

@test "runc run (hooks library tests)" {
	requires root
	requires no_systemd

	# setup some dummy libs
	gcc -shared -Wl,-soname,librunc-hooks-create-runtime.so.1 -o "$HOOKLIBCR.1.0.0"
	gcc -shared -Wl,-soname,librunc-hooks-create-container.so.1 -o "$HOOKLIBCC.1.0.0"

	current_pwd="$(pwd)"

	# To mount $HOOKLIBCR we need to do that in the container namespace
	create_runtime_hook=$(cat <<-EOF
		pid=\$(cat - | jq -r '.pid')
		touch "$LIBPATH/$HOOKLIBCR.1.0.0"
		nsenter -m \$ns -t \$pid mount --bind "$current_pwd/$HOOKLIBCR.1.0.0" "$LIBPATH/$HOOKLIBCR.1.0.0"
	EOF)

	create_container_hook="touch ./lib/$HOOKLIBCC.1.0.0 && mount --bind $current_pwd/$HOOKLIBCC.1.0.0 ./lib/$HOOKLIBCC.1.0.0"

	CONFIG=$(jq --arg create_runtime_hook "$create_runtime_hook" --arg create_container_hook "$create_container_hook" '
		.hooks |= . + {"createRuntime": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", $create_runtime_hook]}]} |
		.hooks |= . + {"createContainer": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", $create_container_hook]}]} |
		.hooks |= . + {"startContainer": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", "ldconfig"]}]} |
		.process.args = ["/bin/sh", "-c", "ldconfig -p | grep librunc"]' $DEBIAN_BUNDLE/config.json)
	echo "${CONFIG}" > config.json

	runc run test_debian
	[ "$status" -eq 0 ]

	echo "Checking create-runtime library"
	echo $output | grep $HOOKLIBCR
	[ "$?" -eq 0 ]

	echo "Checking create-container library"
	echo $output | grep $HOOKLIBCC
	[ "$?" -eq 0 ]
}
