#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run [seccomp -ENOSYS handling]" {
	TEST_NAME="seccomp_syscall_test1"

	# Compile the test binary and update the config to run it.
	gcc -static -o rootfs/seccomp_test "${TESTDATA}/${TEST_NAME}.c"
	update_config ".linux.seccomp = $(<"${TESTDATA}/${TEST_NAME}.json")"
	update_config '.process.args = ["/seccomp_test"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run [seccomp defaultErrnoRet=ENXIO]" {
	TEST_NAME="seccomp_syscall_test2"

	# Compile the test binary and update the config to run it.
	gcc -static -o rootfs/seccomp_test2 "${TESTDATA}/${TEST_NAME}.c"
	update_config ".linux.seccomp = $(<"${TESTDATA}/${TEST_NAME}.json")"
	update_config '.process.args = ["/seccomp_test2"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# TODO:
# - Test other actions like SCMP_ACT_TRAP, SCMP_ACT_TRACE, SCMP_ACT_LOG.
# - Test args (index, value, valueTwo, etc).

@test "runc run [seccomp] (SCMP_ACT_ERRNO default)" {
	update_config '   .process.args = ["/bin/sh", "-c", "mkdir /dev/shm/foo"]
			| .process.noNewPrivileges = false
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32","SCMP_ARCH_X86_64","SCMP_ARCH_AARCH64","SCMP_ARCH_ARM"],
				"syscalls":[{"names":["mkdir","mkdirat"], "action":"SCMP_ACT_ERRNO"}]
			}'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"mkdir:"*"/dev/shm/foo"*"Operation not permitted"* ]]
}

@test "runc run [seccomp] (SCMP_ACT_ERRNO explicit errno)" {
	update_config '   .process.args = ["/bin/sh", "-c", "mkdir /dev/shm/foo"]
			| .process.noNewPrivileges = false
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32","SCMP_ARCH_X86_64","SCMP_ARCH_AARCH64","SCMP_ARCH_ARM"],
				"syscalls":[{"names":["mkdir","mkdirat"], "action":"SCMP_ACT_ERRNO", "errnoRet": 100}]
			}'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"Network is down"* ]]
}

# Prints the numeric value of provided seccomp flags combination.
# The parameter is flags string, as supplied in OCI spec, for example
# '"SECCOMP_FILTER_FLAG_TSYNC","SECCOMP_FILTER_FLAG_LOG"'.
function flags_value() {
	# Numeric values of seccomp flags.
	declare -A values=(
		['"SECCOMP_FILTER_FLAG_TSYNC"']=0 # Supported but ignored by runc, thus 0.
		['"SECCOMP_FILTER_FLAG_LOG"']=2
		['"SECCOMP_FILTER_FLAG_SPEC_ALLOW"']=4
		# XXX: add new values above this line.
	)
	# Split the flags.
	IFS=',' read -ra flags <<<"$1"

	local flag v sum=0
	for flag in "${flags[@]}"; do
		# This will produce "values[$flag]: unbound variable"
		# error for a new flag yet unknown to the test.
		v=${values[$flag]}
		((sum += v)) || true
	done

	echo $sum
}

@test "runc run [seccomp] (SECCOMP_FILTER_FLAG_*)" {
	update_config '   .process.args = ["/bin/sh", "-c", "mkdir /dev/shm/foo"]
			| .process.noNewPrivileges = false
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32","SCMP_ARCH_X86_64","SCMP_ARCH_AARCH64","SCMP_ARCH_ARM"],
				"syscalls":[{"names":["mkdir", "mkdirat"], "action":"SCMP_ACT_ERRNO"}]
			}'

	# Get the list of flags supported by runc/seccomp/kernel,
	# or "null" if no flags are supported or runc is too old.
	mapfile -t flags < <(__runc features | jq -c '.linux.seccomp.supportedFlags' |
		tr -d '[]\n' | tr ',' '\n')

	# This is a set of all possible flag combinations to test.
	declare -A TEST_CASES=(
		['EMPTY']=0  # Special value: empty set of flags.
		['REMOVE']=0 # Special value: no flags set.
	)

	# If supported, runc should set SPEC_ALLOW if no flags are set.
	if [[ " ${flags[*]} " == *' "SECCOMP_FILTER_FLAG_SPEC_ALLOW" '* ]]; then
		TEST_CASES['REMOVE']=$(flags_value '"SECCOMP_FILTER_FLAG_SPEC_ALLOW"')
	fi

	# Add all possible combinations of seccomp flags
	# and their expected numeric values to TEST_CASES.
	if [ "${flags[0]}" != "null" ]; then
		# Use shell {a,}{b,}{c,} to generate the powerset.
		for fc in $(eval echo "$(printf "{'%s,',}" "${flags[@]}")"); do
			# Remove the last comma.
			fc="${fc/%,/}"
			TEST_CASES[$fc]=$(flags_value "$fc")
		done
	fi

	# Finally, run the tests.
	for key in "${!TEST_CASES[@]}"; do
		case "$key" in
		'REMOVE')
			update_config ' del(.linux.seccomp.flags)'
			;;
		'EMPTY')
			update_config ' .linux.seccomp.flags = []'
			;;
		*)
			update_config ' .linux.seccomp.flags = [ '"${key}"' ]'
			;;
		esac

		runc --debug run test_busybox
		[ "$status" -ne 0 ]
		[[ "$output" == *"mkdir:"*"/dev/shm/foo"*"Operation not permitted"* ]]

		# Check the numeric flags value, as printed in the debug log, is as expected.
		exp="\"seccomp filter flags: ${TEST_CASES[$key]}\""
		echo "flags $key, expecting $exp"
		[[ "$output" == *"$exp"* ]]
	done
}

@test "runc run [seccomp] (SCMP_ACT_KILL)" {
	update_config '  .process.args = ["/bin/sh", "-c", "mkdir /dev/shm/foo"]
			| .process.noNewPrivileges = false
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32","SCMP_ARCH_X86_64","SCMP_ARCH_AARCH64","SCMP_ARCH_ARM"],
				"syscalls":[{"names":["mkdir","mkdirat"], "action":"SCMP_ACT_KILL"}]
			}'

	runc run test_busybox
	[ "$status" -ne 0 ]
}

# check that a startContainer hook is run with the seccomp filters applied
@test "runc run [seccomp] (startContainer hook)" {
	update_config '   .process.args = ["/bin/true"]
			| .linux.seccomp = {
				"defaultAction":"SCMP_ACT_ALLOW",
				"architectures":["SCMP_ARCH_X86","SCMP_ARCH_X32","SCMP_ARCH_X86_64","SCMP_ARCH_AARCH64","SCMP_ARCH_ARM"],
				"syscalls":[{"names":["mkdir","mkdirat"], "action":"SCMP_ACT_KILL"}]
			}
			| .hooks = {
				"startContainer": [ {
						"path": "/bin/sh",
						"args": ["sh", "-c", "mkdir /dev/shm/foo"]
				} ]
			}'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"error running startContainer hook"* ]]
	[[ "$output" == *"bad system call"* ]]
}
