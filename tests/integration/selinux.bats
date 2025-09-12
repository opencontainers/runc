#!/usr/bin/env bats

load helpers

function setup() {
	requires root # for chcon
	if ! selinuxenabled; then
		skip "requires SELinux enabled"
	fi

	setup_busybox

	# Use a copy of runc binary with proper selinux label set.
	cp "$RUNC" ./runc
	export RUNC="$PWD/runc"
	chcon -u system_u -r object_r -t container_runtime_exec_t "$RUNC"

	# Label container fs.
	chcon -u system_u -r object_r -t container_file_t -R rootfs

	# Save the start date and time for ausearch.
	AU_DD="$(date +%x)"
	AU_TT="$(date +%H:%M:%S)"
}

function teardown() {
	teardown_bundle
	# Show any avc denials.
	if [[ -v AU_DD && -v AU_TT ]] && command -v ausearch &>/dev/null; then
		ausearch -ts "$AU_DD" "$AU_TT" -i -m avc,user_avc || true
	fi
}

# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function run_check_label() {
	HELPER="key_label"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/

	LABEL="system_u:system_r:container_t:s0:c4,c5"
	update_config '	  .process.selinuxLabel |= "'"$LABEL"'"
			| .process.args = ["/bin/'"$HELPER"'"]'
	runc run tst
	[ "$status" -eq 0 ]
	# Key name is _ses.$CONTAINER_NAME.
	KEY=_ses.tst
	[ "$output" == "$KEY $LABEL" ]
}

# This needs to be placed at the top of the bats file to work around
# a shellcheck bug. See <https://github.com/koalaman/shellcheck/issues/2873>.
function exec_check_label() {
	HELPER="key_label"
	cp "${TESTBINDIR}/${HELPER}" rootfs/bin/

	LABEL="system_u:system_r:container_t:s0:c4,c5"
	update_config '	  .process.selinuxLabel |= "'"$LABEL"'"
			| .process.args = ["/bin/sh"]'
	runc run -d --console-socket "$CONSOLE_SOCKET" tst
	[ "$status" -eq 0 ]

	runc exec tst sh -cx "/bin/$HELPER"
	runc exec tst "/bin/$HELPER"
	[ "$status" -eq 0 ]
	# Key name is _ses.$CONTAINER_NAME.
	KEY=_ses.tst
	[ "$output" == "$KEY $LABEL" ]
}

function enable_userns() {
	update_config '	  .linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
			| .linux.gidMappings += [{"hostID": 200000, "containerID": 0, "size": 65534}]'
	remap_rootfs
}

# Baseline test, to check that runc works with selinux enabled.
@test "runc run (no selinux label)" {
	update_config '	  .process.args = ["/bin/true"]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc run (custom selinux label)" {
	update_config '	  .process.selinuxLabel |= "system_u:system_r:container_t:s0:c4,c5"
			| .process.args = ["/bin/true"]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc run (session keyring security label)" {
	run_check_label
}

@test "runc exec (session keyring security label)" {
	exec_check_label
}

@test "runc run (session keyring security label + userns)" {
	enable_userns
	run_check_label
}

@test "runc exec (session keyring security label + userns)" {
	enable_userns
	exec_check_label
}
