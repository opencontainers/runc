#!/usr/bin/env bash
# Detect if we are running inside bats (i.e. inside integration tests) or just
# called by an end-user
# bats-core v1.2.1 defines BATS_RUN_TMPDIR
if [ -z "$BATS_RUN_TMPDIR" ]; then
	# When not running in bats, we create the config.json
	set -e
	runc spec
fi

# We can't source $(dirname $0)/../../../tests/integration/helpers.bash as that
# exits when not running inside bats. We can do hacks, but just to redefine
# update_config() seems clearer. We don't even really need to keep them in sync.
function update_config() {
	jq "$1" "./config.json" | awk 'BEGIN{RS="";getline<"-";print>ARGV[1]}' "./config.json"
}

update_config '.linux.seccomp = {
                        "defaultAction": "SCMP_ACT_ALLOW",
                        "listenerPath": "/run/seccomp-agent.socket",
                        "listenerMetadata": "foo",
                        "architectures": [ "SCMP_ARCH_X86", "SCMP_ARCH_X32", "SCMP_ARCH_X86_64" ],
                        "syscalls": [
                                {
                                        "names": [ "chmod", "fchmod", "fchmodat", "mkdir" ],
                                        "action": "SCMP_ACT_NOTIFY"
                                }
			]
		}'

update_config '.process.args = [
				"sh",
				"-c",
				"set -x; cd /dev/shm; mkdir test-dir; touch test-file; chmod 777 test-file; stat /dev/shm/test-dir-foo && ls -l /dev/shm && echo \"Note the agent added a suffix for the directory name and chmod fails\" "
				]'
