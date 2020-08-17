#!/bin/bash

IMAGE="$1"
GOARCH=$(go env GOARCH)
# according to documents: https://github.com/docker-library/official-images#architectures-other-than-amd64
case ${GOARCH} in
	arm)
		skopeo copy -q docker://$(arch)/$IMAGE:latest "oci:$IMAGE:latest"
	;;

	*)
		skopeo copy -q docker://$GOARCH/$IMAGE:latest "oci:$IMAGE:latest"
	;;
esac

mkdir "$IMAGE"-tmp
args="$([ $(id -u) != "0" ]&& echo -n "--rootless")"
umoci unpack --image "$IMAGE" $args "$IMAGE"-tmp
cd "$IMAGE"-tmp/rootfs
tar -cf ../../"$IMAGE".tar .
cd ../../
rm -rf "$IMAGE"-tmp

# add tarball read permission
chmod a+r "$IMAGE".tar
