#!/usr/bin/env bats
load helpers

function setup() {
	requires root
	rm /dev/tpmtpmm2 || true
	rm /dev/tpmtpmm12 || true
	rm /dev/tpmtpmstop || true
	rm /dev/tpmtpmkill || true
	rm /dev/tpmtpmdelete || true
	rm /dev/tpmtpmforcekill || true
    setup_debian
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
					  | .linux.resources.vtpms += [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmm2", "vtpmMajor": 100, "vtpmMinor": 1}]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc run (with one tpm1.2 device)" {
    HELPER="tpm-helper"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/'"$HELPER"'", "-devicePath=/dev/tpmtpmm12", "-deviceVersion=1.2"]
					  | .linux.resources.vtpms += [{"statepath": "'"$vtpm_path"'", "vtpmversion": "1.2", "vtpmname" : "tpmm12", "vtpmMajor": 100, "vtpmMinor": 1}]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc pause/resume container with vtpm device" {
    HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms += [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmstop", "vtpmMajor": 100, "vtpmMinor": 1}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst

	runc pause tst
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpmtpmstop -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from swtpm paused container"
	fi
	
	runc resume tst
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpmtpmstop -deviceVersion=2
	ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from swtpm resumed container"
	fi
}

@test "runc kill/delete container with vtpm device" {
    HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms += [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmkill", "vtpmMajor": 100, "vtpmMinor": 1}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst
	
	runc kill tst KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst stopped
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpmtpmkill -deviceVersion=2
	ret=$?
	if [ "$ret" -ne 0 ]; then
		fail "should be able to read from swtpm killed container"
	fi
	
	runc delete tst
	[ "$status" -eq 0 ]
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpmtpmkill -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from swtpm deleted container"
	fi
}

@test "runc force delete container with vtpm device" {
    HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms += [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmdelete", "vtpmMajor": 100, "vtpmMinor": 1}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst
	
	runc delete --force tst
	[ "$status" -eq 0 ]
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpmtpmdelete -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from swtpm deleted container"
	fi
}


@test "runc kill swtpm process" {
	HELPER="tpm-helper"
	vtpm_path=$(mktemp -d)
	update_config '	  .process.args = ["/bin/sh"]
					  |.linux.resources.vtpms += [{"statepath": "'"$vtpm_path"'", "vtpmversion": "2", "vtpmname" : "tpmforcekill", "vtpmMajor": 100, "vtpmMinor": 1}]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]
	wait_for_container 10 1 tst
	
	swtpm_pid=$(cat $vtpm_path/tpmforcekill-swtpm.pid)
	kill -9 "$swtpm_pid"
	${TESTBINDIR}/${HELPER} -devicePath=/dev/tpmtpmforcekill -deviceVersion=2 || ret=$?
	if [ "$ret" -ne 1 ]; then
		fail "should not be able to read from killed swtpm"
	fi
}