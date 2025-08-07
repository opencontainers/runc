#!/usr/bin/env bats

load helpers

function create_netns() {
	# Create a temporary name for the test network namespace.
	tmp=$(mktemp -u)
	ns_name=$(basename "$tmp")

	# Create the network namespace.
	ip netns add "$ns_name"
	ns_path=$(ip netns add "$ns_name" 2>&1 | sed -e 's/.*"\(.*\)".*/\1/')
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

	runc run test_busybox
	[ "$status" -eq 0 ]
}

@test "move network device to container network namespace and restore it back" {
	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	update_config ' .linux.netDevices |= {"dummy0": {} }'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# The network namespace owner controls the lifecycle of the interface.
	# The interface should remain on the namespace after the container was killed.
	runc delete --force test_busybox

	# Move back the interface to the root namespace (pid 1).
	ip netns exec "$ns_name" ip link set dev dummy0 netns 1

	# Verify the interface is back in the root network namespace.
	ip address show dev dummy0
}

@test "move network device to precreated container network namespace" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# Verify the interface is still present in the network namespace.
	ip netns exec "$ns_name" ip address show dev dummy0
}

@test "move network device to precreated container network namespace and set ip address with global scope" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	global_ip="169.254.169.77/32"

	# Set the interface down to avoid possible network problems.
	# Set a custom address to the interface.
	ip link set down dev dummy0
	ip address add "$global_ip" dev dummy0

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"$global_ip "* ]]

	# Verify the interface is still present in the network namespace.
	run ip netns exec "$ns_name" ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"$global_ip "* ]]
}

@test "move network device to precreated container network namespace and set ip address without global scope" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	non_global_ip="127.0.0.33"

	# Set the interface down to avoid possible network problems.
	# Set a custom address to the interface.
	ip link set down dev dummy0
	ip address add "$non_global_ip" dev dummy0

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" != *" $non_global_ip "* ]]

	# Verify the interface is still present in the network namespace.
	ip netns exec "$ns_name" ip address show dev dummy0
}

@test "move network device to precreated container network namespace and set mtu" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	mtu_value=1789
	# Cet a custom mtu to the interface.
	ip link set mtu "$mtu_value" dev dummy0

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"mtu $mtu_value "* ]]

	# Verify the interface is still present in the network namespace.
	run ip netns exec "$ns_name" ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"mtu $mtu_value "* ]]
}

@test "move network device to precreated container network namespace and set mac address" {
	update_config ' .linux.netDevices |= {"dummy0": {} }
      		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	mac_address="00:11:22:33:44:55"
	# set a custom mac address to the interface
	ip link set address "$mac_address" dev dummy0

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *"ether $mac_address "* ]]

	# Verify the interface is still present in the network namespace.
	run ip netns exec "$ns_name" ip address show dev dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *"ether $mac_address "* ]]
}

@test "move network device to precreated container network namespace and rename" {
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
      		| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# Verify the interface is still present in the network namespace.
	ip netns exec "$ns_name" ip address show dev ctr_dummy0
}

@test "move network device to precreated container network namespace and rename and set mtu and mac and ip address" {
	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
	    		| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	# Tell runc which network namespace to use.
	create_netns
	update_config '(.. | select(.type? == "network")) .path |= "'"$ns_path"'"'

	mtu_value=1789
	mac_address="00:11:22:33:44:55"
	global_ip="169.254.169.77/32"

	# Set a custom mtu to the interface.
	ip link set mtu "$mtu_value" dev dummy0
	# Set a custom mac address to the interface.
	ip link set address "$mac_address" dev dummy0
	# Set a custom ip address to the interface.
	ip address add "$global_ip" dev dummy0

	runc run test_busybox
	[ "$status" -eq 0 ]
	[[ "$output" == *" $global_ip "* ]]
	[[ "$output" == *"ether $mac_address "* ]]
	[[ "$output" == *"mtu $mtu_value "* ]]

	# Verify the interface is still present in the network namespace.
	run ip netns exec "$ns_name" ip address show dev ctr_dummy0
	[ "$status" -eq 0 ]
	[[ "$output" == *" $global_ip "* ]]
	[[ "$output" == *"ether $mac_address "* ]]
	[[ "$output" == *"mtu $mtu_value "* ]]
}
