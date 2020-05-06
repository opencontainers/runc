#!/bin/bash
get_busybox(){
	case $(go env GOARCH) in
	arm64)
		echo 'https://github.com/docker-library/busybox/raw/dist-arm64v8/glibc/busybox.tar.xz'
	;;
	s390x)
		echo 'https://github.com/docker-library/busybox/raw/dist-s390x/glibc/busybox.tar.xz'
	;;
	ppc64le)
		echo 'https://github.com/docker-library/busybox/raw/dist-ppc64le/glibc/busybox.tar.xz'
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
	s390x)
		echo 'hello-world-s390x.tar'
	;;
	ppc64le)
		echo 'hello-world-ppc64le.tar'
	;;
        *)
                echo 'hello-world.tar'
        ;;
        esac
}
