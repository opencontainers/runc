#!/usr/bin/env bats
load helpers

function setup() {
	# Root privileges are required by swtpm_cuse.
	# https://github.com/stefanberger/swtpm/blob/master/src/swtpm/cuse_tpm.c#L1806
	requires root
	rm /etc/swtpm/runc.conf || true
    setup_debian
	ROOT_HASH=$(sha256sum - <<<"$ROOT/state")
	ROOT_HASH_OFFSET=${ROOT_HASH:0:6}
	test_major=${RUN_IN_CONTAINER_MAJOR:-0}
	test_major_second=${RUN_IN_CONTAINER_MAJOR_SECOND:-0}
	test_minor=${RUN_IN_CONTAINER_MINOR:-0}
}

function teardown() {
    teardown_bundle
}

@test "runc run (with no tpm device)" {
    HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	update_config '	  .process.args = ["/bin/'"$HELPER"'"]'
	runc run tst
	[ "$status" -ne 0 ]
}

@test "runc run (with one tpm2 device)" {
    HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/'"$HELPER"'", "-devicePath=/dev/tpmtpmm2"]
					  | .linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmm2", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc run (with one tpm1.2 device)" {
    HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/'"$HELPER"'", "-devicePath=/dev/tpmtpmm12", "-deviceVersion=1.2"]
					  | .linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "1.2", "vtpmname" : "tpmm12", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc pause/resume container with vtpm device" {
    HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmstop", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst

	runc pause tst
	
	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmstop -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from swtpm paused container"
	fi
	
	runc resume tst
	
	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmstop -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from swtpm resumed container"
	fi
}

@test "runc kill/delete container with vtpm device" {
    HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmkill", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst
	
	runc kill tst KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst stopped
	
	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmkill -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from swtpm killed container"
	fi
	
	runc delete tst
	[ "$status" -eq 0 ]

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmkill -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from swtpm deleted container"
	fi
}

@test "runc force delete container with vtpm device" {
    HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmdelete", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst
	
	runc delete --force tst
	[ "$status" -eq 0 ]

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmdelete -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from swtpm deleted container"
	fi
}


@test "runc kill swtpm process" {
	HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmforcekill", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst
	swtpm_pid=$(cat $vtpm_path/"$ROOT_HASH_OFFSET"-tst-tpmforcekill-swtpm.pid)
	kill -9 "$swtpm_pid"

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmforcekill -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from killed swtpm"
	fi
	runc delete --force tst
	[ "$status" -eq 0 ]
}


@test "runc with 2 container with the same devpath" {
	HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	# first container
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmsame", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst1
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst1

	# second container
	vtpm_pth1=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_pth1"'", "vtpmversion": "2", "vtpmname" : "tpmsame", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}]'

	runc run -d --console-socket "$CONSOLE_SOCKET" tst2
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst2

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst1-tpmsame -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from first container"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst2-tpmsame -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from second container"
	fi

	runc exec tst1 "${HELPER}" -devicePath=/dev/tpmtpmsame -deviceVersion=2
	[ "$status" -eq 0 ]
	
	runc exec tst2 "${HELPER}" -devicePath=/dev/tpmtpmsame -deviceVersion=2
	[ "$status" -eq 0 ]

	runc delete --force tst1
	[ "$status" -eq 0 ]

	runc exec tst2 "${HELPER}" -devicePath=/dev/tpmtpmsame -deviceVersion=2
	[ "$status" -eq 0 ]
}

@test "runc run with wrong VTPM params" {
	HELPER="tpm-helper"
	vtpm_path1=$(mktemp -d)
	vtpm_path2=$(mktemp -d)
	vtpm_path3=$(mktemp -d)

	# empty name
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path1"'", "vtpmversion": "2", "vtpmname" : "", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -ne 0 ]

	# the same name
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path2"'", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'},
					  							{"statepath": "'"$vtpm_path3"'", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}
					  ]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -ne 0 ]

	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path1"'", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst1
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst1

	# with the same state path
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path1"'", "vtpmversion": "2", "vtpmname" : "tpmsecond", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst2
	[ "$status" -ne 0 ]

	# with the same major/minor. major and minor are not bind to some values (unless we are running in the container).
	# we need to know the right major/minor of tst1 vtpm device.
	the_same_minor=$(ls -la /dev/tpm"$ROOT_HASH_OFFSET"-tst1-tpmone | awk '{print $6}')
	the_same_major_str=$(ls -la /dev/tpm"$ROOT_HASH_OFFSET"-tst1-tpmone | awk '{print $5}')
	the_same_major=${the_same_major_str::-1}
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path2"'", "vtpmversion": "2", "vtpmname" : "tpmsecond", "vtpmMajor": '"$the_same_major"', "vtpmMinor": '"$the_same_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst3
	[ "$status" -ne 0 ]

	# with different params
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path3"'", "vtpmversion": "2", "vtpmname" : "tpmsecond", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst4
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst4
}


@test "runc run container with 2 devices" {
	HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path1=$(mktemp -d)
	vtpm_path2=$(mktemp -d)

	# two devices
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path1"'", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'},
					  							 {"statepath": "'"$vtpm_path2"'", "vtpmversion": "2", "vtpmname" : "tpmsecond", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}
					  ]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]

	runc exec tst "${HELPER}" -devicePath=/dev/tpmtpmone -deviceVersion=2
	[ "$status" -eq 0 ]
	
	runc exec tst "${HELPER}" -devicePath=/dev/tpmtpmsecond -deviceVersion=2
	[ "$status" -eq 0 ]

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmone -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from first device"
	fi
	
	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsecond -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from second device"
	fi
}

@test "runc run with ignored errors" {
	mkdir -p /etc/swtpm
	echo '{"ignoreVTPMErrors": true}' > /etc/swtpm/runc.conf
	HELPER="tpm-helper"
	vtpm_path1=$(mktemp -d)
	vtpm_path2=$(mktemp -d)
	vtpm_path3=$(mktemp -d)
	vtpm_path4=$(mktemp -d)

	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path1"'", "vtpmversion": "2", "vtpmname" : "", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst1
	[ "$status" -eq 0 ]

	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path2"'", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'},
					  							 {"statepath": "'"$vtpm_path3"'", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}
					  ]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst2
	[ "$status" -eq 0 ]


	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst1- -deviceVersion=2 || ret=$?
	if [ "$ret" -eq 0 ]; then
		fail "should not be able to read from empty name device"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst2-tpmon2 -deviceVersion=2 || ret=$?
	if [ "$ret" -eq 0 ]; then
		fail "should not be able to read from repeated name device"
	fi

	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms = [{"statepath": "", "vtpmversion": "2", "vtpmname" : "tpmone", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'},
					  							 {"statepath": "'"$vtpm_path4"'", "vtpmversion": "2", "vtpmname" : "tpmsecond", "vtpmMajor": '"$test_major_second"', "vtpmMinor": '"$test_minor"'}
					  ]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst3
	[ "$status" -eq 0 ]

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmone -deviceVersion=2 || ret=$?
	if [ "$ret" -eq 0 ]; then
		fail "should not be able to read from wrong major device"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmsecond -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from right name device"
	fi
}

@test "runc swtpm with user namespace" {
	rm /etc/swtpm/runc.conf || true
	HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.namespaces += [{"type": "user"}]
					  |.linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
					  |.linux.gidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
					  |.linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmuser", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	remap_rootfs
	
	runc run -d --console-socket "$CONSOLE_SOCKET" tst1
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst1

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst1-tpmuser -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from user namespaced container"
	fi

	runc exec tst1 "${HELPER}" -devicePath=/dev/tpmtpmuser -deviceVersion=2
	[ "$status" -ne 0 ]

	runc exec tst1 "${HELPER}" -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst1-tpmuser -deviceVersion=2
	[ "$status" -eq 0 ]

	runc delete --force tst1
	[ "$status" -eq 0 ]
}

@test "runc swtpm with joined user namespace" {
	HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path=$(mktemp -d)
	update_config '	.process.args = ["sleep", "infinity"]
					|.linux.namespaces += [{"type": "user"}]
					|.linux.uidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]
					|.linux.gidMappings += [{"containerID": 0, "hostID": 100000, "size": 65536}]'
	
	runc run -d --console-socket "$CONSOLE_SOCKET" tmp_sleep_container
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tmp_sleep_container

	target_pid="$(__runc state tmp_sleep_container | jq .pid)"
	update_config '.linux.namespaces |= map(if .type == "user" then (.path = "/proc/'"$target_pid"'/ns/" + .type) else . end)
					| del(.linux.uidMappings)
					| del(.linux.gidMappings)
					| .linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "joined_userns", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	
	runc run -d --console-socket "$CONSOLE_SOCKET" vtpm_in_joined_userns
	[ "$status" -eq 0 ]
	wait_for_container 10 1 vtpm_in_joined_userns

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-vtpm_in_joined_userns-joined_userns -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from user namespaced container"
	fi

	runc exec vtpm_in_joined_userns "${HELPER}" -devicePath=/dev/tpmjoined_userns -deviceVersion=2
	[ "$status" -ne 0 ]

	runc exec vtpm_in_joined_userns "${HELPER}" -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-vtpm_in_joined_userns-joined_userns -deviceVersion=2
	[ "$status" -eq 0 ]
}

@test "runc check swtpm_setup" {
	HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path=$(mktemp -d)

	update_config '	  .process.args = ["/bin/sh"]
					  | .linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "statePathIsManaged": true, "vtpmversion": "2", "createCerts": true, "pcrBanks": "sha256,sha1", "encryptionPassword": "12345", "vtpmname" : "tpmsetup", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsetup -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsetup -deviceVersion=2 -deviceCommand=pcr -pcrs=10,16 -hashAlgo=sha256 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read activated pcrs from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsetup -deviceVersion=2 -deviceCommand=pcr -pcrs=10,16 -hashAlgo=sha1 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read activated pcrs from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsetup -deviceVersion=2 -deviceCommand=pcr -pcrs=10,16 -hashAlgo=sha512 || ret=$?
	if [ "$ret" -eq 0 ]; then
		fail "should not be able to read deactivated pcrs from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsetup -deviceVersion=2 -deviceCommand=pubek || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read pubek from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst-tpmsetup -deviceVersion=2 -deviceCommand=cert || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read ek cert from container created by swtpm_setup"
	fi

	runc delete --force tst
	[ "$status" -eq 0 ]

	# wrong encryption password
	update_config '	  .process.args = ["/bin/sh"]
					  | .linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "statePathIsManaged": true, "vtpmversion": "2", "createCerts": true, "pcrBanks": "sha256,sha1", "encryptionPassword": "54321", "vtpmname" : "tpmsetup", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst1
	[ "$status" -ne 0 ]

	# password with encryption
	update_config '	  .process.args = ["/bin/sh"]
					  | .linux.resources.vtpms = [{"statepath": "'"$vtpm_path"'", "statePathIsManaged": true, "vtpmversion": "2", "createCerts": true, "pcrBanks": "sha256,sha1", "encryptionPassword": "pass=12345", "vtpmname" : "tpmsetup", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst2
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst2

	runc delete --force tst2
	[ "$status" -eq 0 ]

	vtpm_path1=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					| .linux.resources.vtpms = [{"statepath": "'"$vtpm_path1"'", "statePathIsManaged": true, "vtpmversion": "1.2", "createCerts": true, "encryptionPassword": "12345", "vtpmname" : "tpmsetup", "vtpmMajor": '"$test_major"', "vtpmMinor": '"$test_minor"'}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst3
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst3

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmsetup -deviceVersion=1.2 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmsetup -deviceVersion=1.2 -deviceCommand=pcr -pcrs=10,16 || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read activated pcrs from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmsetup -deviceVersion=1.2 -deviceCommand=pubek || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read pubek from container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmsetup -deviceVersion=1.2 -deviceCommand=owner || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to set owner to container created by swtpm_setup"
	fi

	ret=0
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpm"$ROOT_HASH_OFFSET"-tst3-tpmsetup -deviceVersion=1.2 -deviceCommand=cert || ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read ek certs from container created by swtpm_setup"
	fi
}
