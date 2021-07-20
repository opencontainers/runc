#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Prepare source folders for bind mount
	mkdir -p source-{accessible,inaccessible}/dir
	touch source-{accessible,inaccessible}/dir/foo.txt
	chmod 750 source-inaccessible
	mkdir -p rootfs/{proc,sys,tmp}
	mkdir -p rootfs/tmp/mount

	# We need to give permissions for others so the uid inside the userns
	# can mount the rootfs on itself.
	# If we don't do this, the root has permissions only for the owner
	# (root) and we can't read anything inside that dir.
	chmod 755 "$ROOT"

	if [ "$ROOTLESS" -eq 0 ]; then
		update_config ' .linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
			| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '
	fi
}

function teardown() {
	teardown_bundle
}

@test "userns with simple mount" {
	update_config ' .process.args += ["-c", "stat /tmp/mount/foo.txt"]
		| .mounts += [{"source": "source-accessible/dir", "destination": "/tmp/mount", "options": ["bind"]}] '

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "userns with inaccessible mount" {
	update_config ' .process.args += ["-c", "stat /tmp/mount/foo.txt"]
		| .mounts += [{"source": "source-inaccessible/dir", "destination": "/tmp/mount", "options": ["bind"]}] '

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# exec + bindmounts + user ns is a special case in the code. Test that it works.
@test "userns with inaccessible mount + exec" {
	update_config ' .mounts += [{"source": "source-inaccessible/dir", "destination": "/tmp/mount", "options": ["bind"]}] '

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --pid-file pid.txt test_busybox stat /tmp/mount/foo.txt
	[ "$status" -eq 0 ]
}
