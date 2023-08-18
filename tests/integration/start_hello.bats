#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	update_config '.process.args = ["/bin/echo", "Hello World"]'
}

function teardown() {
	teardown_bundle
}

@test "runc run" {
	# run hello-world
	runc run test_hello
	[ "$status" -eq 0 ]

	# check expected output
	[[ "${output}" == *"Hello"* ]]
}

@test "runc run ({u,g}id != 0)" {
	# cannot start containers as another user in rootless setup without idmap
	[ $EUID -ne 0 ] && requires rootless_idmap

	# replace "uid": 0 with "uid": 1000
	# and do a similar thing for gid.
	update_config ' (.. | select(.uid? == 0)) .uid |= 1000
		| (.. | select(.gid? == 0)) .gid |= 100'

	# run hello-world
	runc run test_hello
	[ "$status" -eq 0 ]

	# check expected output
	[[ "${output}" == *"Hello"* ]]
}

# https://github.com/opencontainers/runc/issues/3715.
#
# Fails when using Go 1.20 < 1.20.2, the reasons is https://go.dev/issue/58552.
@test "runc run as user with no exec bit but CAP_DAC_OVERRIDE set" {
	requires root # Can't chown/chmod otherwise.

	# Remove exec perm for everyone but owner (root).
	chown 0 rootfs/bin/echo
	chmod go-x rootfs/bin/echo

	# Replace "uid": 0 with "uid": 1000 and do a similar thing for gid.
	update_config '	  (.. | select(.uid? == 0)) .uid |= 1000
			| (.. | select(.gid? == 0)) .gid |= 100'

	# Sanity check: make sure we can't run the container w/o CAP_DAC_OVERRIDE.
	runc run test_busybox
	[ "$status" -ne 0 ]

	# Enable CAP_DAC_OVERRIDE.
	update_config '	  .process.capabilities.bounding += ["CAP_DAC_OVERRIDE"]
			| .process.capabilities.effective += ["CAP_DAC_OVERRIDE"]
			| .process.capabilities.inheritable += ["CAP_DAC_OVERRIDE"]
			| .process.capabilities.ambient += ["CAP_DAC_OVERRIDE"]
			| .process.capabilities.permitted += ["CAP_DAC_OVERRIDE"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "runc run with rootfs set to ." {
	cp config.json rootfs/.
	rm config.json
	cd rootfs
	update_config '(.. | select(. == "rootfs")) |= "."'

	# run hello-world
	runc run test_hello
	[ "$status" -eq 0 ]
	[[ "${output}" == *"Hello"* ]]
}

@test "runc run --pid-file" {
	# run hello-world
	runc run --pid-file pid.txt test_hello
	[ "$status" -eq 0 ]
	[[ "${output}" == *"Hello"* ]]

	# check pid.txt was generated
	[ -e pid.txt ]

	[[ "$(cat pid.txt)" =~ [0-9]+ ]]
}

# https://github.com/opencontainers/runc/pull/2897
@test "runc run [rootless with host pidns]" {
	requires rootless_no_features

	# Remove pid namespace, and replace /proc mount
	# with a bind mount from the host.
	update_config '	  .linux.namespaces -= [{"type": "pid"}]
			| .mounts |= map((select(.type == "proc")
				| .type = "none"
				| .source = "/proc"
				| .options = ["rbind", "nosuid", "nodev", "noexec"]
			  ) // .)'

	runc run test_hello
	[ "$status" -eq 0 ]
}

@test "runc run [redundant seccomp rules]" {
	update_config '	  .linux.seccomp = {
				"defaultAction": "SCMP_ACT_ALLOW",
				"syscalls": [{
					"names": ["bdflush"],
					"action": "SCMP_ACT_ALLOW",
				}]
			    }'
	runc run test_hello
	[ "$status" -eq 0 ]
}
