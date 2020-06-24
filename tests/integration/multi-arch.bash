#! /bin/bash

get_and_extract_ubuntu() {
	local cache="/tmp/ubuntu-cache"
	local ubuntu="ubuntu:latest"
	local rootless=$(id -u)

	if [ "$rootless" -ne 0 ]; then
		cache="/tmp/ubuntu-cache-rootless"
	fi

	mkdir -p "$cache"
	cd "$cache" || return

	if [ ! -d "$cache/ubuntu" ]; then
		case $(go env GOARCH) in
		arm64)
			skopeo copy docker://arm64v8/ubuntu:focal "oci:$ubuntu"
		;;
		*)
			skopeo copy docker://ubuntu:focal "oci:$ubuntu"
		;;
		esac
	fi

	if [ ! -d "$cache/rootfs" ]; then
		if [ "$rootless" -ne 0 ]; then
			umoci unpack --rootless --image "$ubuntu" "$cache"
		else
			umoci unpack --image "$ubuntu" "$cache"
		fi
	fi

	rm -r -f "$1"
	cp -a $cache "$1"
	cd - || return
}
