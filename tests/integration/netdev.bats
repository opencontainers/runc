#!/usr/bin/env bats

load helpers

function setup_netns() {
	local tmp

	# Create a temporary name for the test network namespace.
	tmp=$(mktemp -u)
	ns_name=$(basename "$tmp")

	# Create the network namespace.
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')

	# Tell runc to use it.
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'
}

function delete_netns() {
	# Delete the namespace only if the ns_name variable is set.
	[ -v ns_name ] && ip netns del "$ns_name"
}

function setup() {
	requires root
	setup_busybox

	# Create a dummy interface to move to the container.
	ip link add dummy0 type dummy
}

function teardown() {
	ip link del dev dummy0
	delete_netns
	teardown_bundle
}

@test "move network device to container network namespace" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	runc -0 run test_busybox
}

@test "move network device to container network namespace and restore it back" {
	setup_netns
	update_config ' .linux.netDevices |= {"dummy0": {} }'

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	# The network namespace owner controls the lifecycle of the interface.
	# The interface should remain on the namespace after the container was killed.
	runc -0 delete --force test_busybox

	# Move back the interface to the root namespace (pid 1).
	ip netns exec "$ns_name" ip link set dev dummy0 netns 1

	# Verify the interface is back in the root network namespace.
	ip address show dev dummy0
}

@test "move network device to precreated container network namespace" {
	setup_netns
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	runc -0 run test_busybox

	# Verify the interface is still present in the network namespace.
	ip netns exec "$ns_name" ip address show dev dummy0
}

@test "move network device to precreated container network namespace and set ip address with global scope" {
	setup_netns
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	global_ip="169.254.169.77/32"

	# Set the interface down to avoid possible network problems.
	# Set a custom address to the interface.
	ip link set down dev dummy0
	ip address add "$global_ip" dev dummy0

	runc -0 run test_busybox
	[[ "$output" == *"$global_ip "* ]]

	# Verify the interface is still present in the network namespace.
	run -0 ip netns exec "$ns_name" ip address show dev dummy0
	[[ "$output" == *"$global_ip "* ]]
}

@test "move network device to precreated container network namespace and set ip address without global scope" {
	setup_netns
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	non_global_ip="127.0.0.33"

	# Set the interface down to avoid possible network problems.
	# Set a custom address to the interface.
	ip link set down dev dummy0
	ip address add "$non_global_ip" dev dummy0

	runc -0 run test_busybox
	[[ "$output" != *" $non_global_ip "* ]]

	# Verify the interface is still present in the network namespace.
	ip netns exec "$ns_name" ip address show dev dummy0
}

@test "move network device to precreated container network namespace and set mtu" {
	setup_netns
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	mtu_value=1789
	# Set a custom mtu to the interface.
	ip link set mtu "$mtu_value" dev dummy0

	runc -0 run test_busybox
	[[ "$output" == *"mtu $mtu_value "* ]]

	# Verify the interface is still present in the network namespace.
	run -0 ip netns exec "$ns_name" ip address show dev dummy0
	[[ "$output" == *"mtu $mtu_value "* ]]
}

@test "move network device to precreated container network namespace and set mac address" {
	setup_netns
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	mac_address="00:11:22:33:44:55"
	# set a custom mac address to the interface
	ip link set address "$mac_address" dev dummy0

	runc -0 run test_busybox
	[[ "$output" == *"ether $mac_address "* ]]

	# Verify the interface is still present in the network namespace.
	run -0 ip netns exec "$ns_name" ip address show dev dummy0
	[[ "$output" == *"ether $mac_address "* ]]
}

@test "move network device to precreated container network namespace and rename" {
	setup_netns
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
      		| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	runc -0 run test_busybox

	# Verify the interface is still present in the network namespace.
	ip netns exec "$ns_name" ip address show dev ctr_dummy0
}

@test "move network device to precreated container network namespace and rename and set mtu and mac and ip address" {
	setup_netns
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
	    		| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	mtu_value=1789
	mac_address="00:11:22:33:44:55"
	global_ip="169.254.169.77/32"

	# Set a custom mtu to the interface.
	ip link set mtu "$mtu_value" dev dummy0
	# Set a custom mac address to the interface.
	ip link set address "$mac_address" dev dummy0
	# Set a custom ip address to the interface.
	ip address add "$global_ip" dev dummy0

	runc -0 run test_busybox
	[[ "$output" == *" $global_ip "* ]]
	[[ "$output" == *"ether $mac_address "* ]]
	[[ "$output" == *"mtu $mtu_value "* ]]

	# Verify the interface is still present in the network namespace.
	run -0 ip netns exec "$ns_name" ip address show dev ctr_dummy0
	[[ "$output" == *" $global_ip "* ]]
	[[ "$output" == *"ether $mac_address "* ]]
	[[ "$output" == *"mtu $mtu_value "* ]]
}
