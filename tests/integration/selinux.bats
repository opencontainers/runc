#!/usr/bin/env bats

load helpers

function setup() {
	requires root # for chcon
	if ! selinuxenabled; then
		skip "requires SELinux enabled and in enforcing mode"
	fi

	setup_busybox

	# Use a copy of runc binary with proper selinux label set.
	cp "$RUNC" .
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
		ausearch -ts "$AU_DD" "$AU_TT" -i -m avc || true
	fi
}

# Baseline test, to check that runc works with selinux enabled.
@test "runc run (no selinux label)" {
	update_config '	  .process.args = ["/bin/true"]'
	runc run tst
	[ "$status" -eq 0 ]
}

# https://github.com/opencontainers/runc/issues/4057
@test "runc run (custom selinux label)" {
	update_config '	  .process.selinuxLabel |= "system_u:system_r:container_t:s0:c4,c5"
			| .process.args = ["/bin/true"]'
	runc run tst
	[ "$status" -eq 0 ]
}

@test "runc run (custom selinux label, RUNC_DMZ=legacy)" {
	export RUNC_DMZ=legacy
	update_config '	  .process.selinuxLabel |= "system_u:system_r:container_t:s0:c4,c5"
			| .process.args = ["/bin/true"]'
	runc run tst
	[ "$status" -eq 0 ]
}
