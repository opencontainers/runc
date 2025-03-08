#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_busybox
	# create a dummy interface to move to the container
	ip link add dummy0 type dummy
}

function teardown() {
	ip link del dev dummy0
	teardown_bundle
}
@test "move network device to container network namespace" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
								| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# TODO(aojea) the network namespace is delete before the interfaces can be restored
	# ip address show dev dummy0
	# [ "$status" -neq 0 ]
}

@test "move network device to precreated container network namespace" {
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

	# verify the interface is returned to the root namespace
	ip address show dev dummy0
	[ "$status" -eq 0 ]
}

@test "move network device to precreated container network namespace and set ip address" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
								| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# set a custom address to the interface
	# set the interface down to avoid network problems
	ip link set down dev dummy0
	ip address add 169.254.169.77 dev dummy0

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
	[[ "$output" == *"169.254.169.77"* ]]

	ip netns del "$ns_name"

	# verify the interface is returned to the root namespace
	ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"169.254.169.77"* ]]
}

@test "move network device to precreated container network namespace and set mtu" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
								| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# set a custom mtu to the interface
	ip link set mtu 1789 dev dummy0

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
	[[ "$output" == *"mtu 1789"* ]]

	ip netns del "$ns_name"

	# verify the interface is returned to the root namespace
	ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"mtu 1789"* ]]
}

@test "move network device to precreated container network namespace and set mac address" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
								| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# set a custom mac address to the interface
	ip link set address 00:11:22:33:44:55 dev dummy0

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
	[[ "$output" == *"ether 00:11:22:33:44:55"* ]]

	ip netns del "$ns_name"

	# verify the interface is returned to the root namespace
	ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"ether 00:11:22:33:44:55"* ]]
}

@test "move network device to precreated container network namespace and rename" {
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

	# verify the interface is returned to the root namespace
	ip address show dev dummy0
	[ "$status" -eq 0 ]
}

@test "move network device to precreated container network namespace and rename and set mtu and mac and ip address" {
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
								| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	# set a custom mtu to the interface
	ip link set mtu 1789 dev dummy0
	# set a custom mac address to the interface
	ip link set address 00:11:22:33:44:55 dev dummy0
	# set a custom ip address to the interface
	ip address add 169.254.169.78 dev dummy0

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
	[[ "$output" == *"169.254.169.78"* ]]
	[[ "$output" == *"ether 00:11:22:33:44:55"* ]]
	[[ "$output" == *"mtu 1789"* ]]

	ip netns del "$ns_name"

	# verify the interface is returned to the root namespace
	ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"169.254.169.78"* ]]
	[[ "$output" == *"ether 00:11:22:33:44:55"* ]]
	[[ "$output" == *"mtu 1789"* ]]
}
