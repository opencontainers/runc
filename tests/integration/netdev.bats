#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_busybox
	# create a dummy interface to move to the container
	ip link add dummy0 type dummy
	ip link set up dev dummy0
	ip addr add 169.254.169.13/32 dev dummy0
}

function teardown() {
	ip link del dev dummy0
	teardown_bundle
}

@test "move network device to container network namespace" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
								| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# create a temporary name for the test network namespace
	tmp=$(mktemp)
	rm -f "$tmp"
	ns_name=$(basename "$tmp")
	# create network namespace
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')

	# tell runc which network namespace to use
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc run test_busybox
	[ "$status" -eq 0 ]

	ip netns del "$ns_name"
}

@test "move network device to container network namespace and rename" {
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
								| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	# create a temporary name for the test network namespace
	tmp=$(mktemp)
	rm -f "$tmp"
	ns_name=$(basename "$tmp")
	# create network namespace
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')

	# tell runc which network namespace to use
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc run test_busybox
	[ "$status" -eq 0 ]

	ip netns del "$ns_name"
}

@test "move network device to container network namespace and change ipv4 address" {
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" , "addresses" : [ "10.0.0.2/24" ]} }
								| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0" ]'

	# create a temporary name for the test network namespace
	tmp=$(mktemp)
	rm -f "$tmp"
	ns_name=$(basename "$tmp")
	# create network namespace
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')

	# tell runc which network namespace to use
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"10.0.0.2/24"* ]]

	ip netns del "$ns_name"
}

@test "move network device to container network namespace and change ipv6 address" {
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" , "addresses" : [ "10.0.0.2/24" , "2001:db8::2/64" ]} }
								| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0" ]'

	# create a temporary name for the test network namespace
	tmp=$(mktemp)
	rm -f "$tmp"
	ns_name=$(basename "$tmp")
	# create network namespace
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')

	# tell runc which network namespace to use
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"2001:db8::2/64"* ]]

	ip netns del "$ns_name"
}

@test "network device on root namespace fails" {
	update_config ' .linux.netDevices |= {"dummy0": {} }'
	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"unable to move network devices without a private NET namespace"* ]]
}

@test "network device bad address fails" {
	update_config '(.. | select(.type? == "network")) .path |= "'fake_net_ns'"'
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" , "addresses" : [ "wrong_ip" ]} }'

	runc run test_busybox
	[ "$status" -ne 0 ]
	[[ "$output" == *"invalid network IP address"* ]]
}
