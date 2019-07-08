#!/bin/bash
get_busybox(){
	case $(go env GOARCH) in
	arm64)
		echo 'https://github.com/docker-library/busybox/raw/dist-arm64v8/glibc/busybox.tar.xz'
	;;
	*)
		echo 'https://github.com/docker-library/busybox/raw/dist-amd64/glibc/busybox.tar.xz'
	;;
	esac
}

get_hello(){
	case $(go env GOARCH) in
        arm64)
                echo 'hello-world-aarch64.tar'
        ;;
        *)
                echo 'hello-world.tar'
        ;;
        esac
}
