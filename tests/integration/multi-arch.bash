#!/bin/bash
get_busybox() {
	case $(go env GOARCH) in
	arm64)
		echo 'https://github.com/docker-library/busybox/raw/dist-arm64v8/glibc/busybox.tar.xz'
	;;
	*)
		echo 'https://github.com/docker-library/busybox/raw/dist-amd64/glibc/busybox.tar.xz'
	;;
	esac
}

get_hello() {
	case $(go env GOARCH) in
	arm64)
		echo 'hello-world-aarch64.tar'
	;;
	*)
		echo 'hello-world.tar'
	;;
	esac
}

get_and_extract_debian() {
	tmp=$(mktemp -d)
	cd "$tmp"

	debian="debian:3.11.6"

	case $(go env GOARCH) in
	arm64)
		skopeo copy docker://arm64v8/debian:buster "oci:$debian"
	;;
	*)
		skopeo copy docker://amd64/debian:buster "oci:$debian"
	;;
	esac

	args="$([ -z "${ROOTLESS_TESTPATH+x}" ] && echo "--rootless")"
	umoci unpack $args --image "$debian" "$1"

	cd -
	rm -rf "$tmp"
}
