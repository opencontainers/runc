#!/bin/bash
set -eux -o pipefail

VERSION=2.5.0
MD5SUM=463b688bf7d227325b5a465b6bdc3ec4

if [ -z ${@+x} ]; then
	HOSTS=(
		"x86_64-linux-gnu"
		"arm-linux-gnueabi"
		"arm-linux-gnueabihf"
		"aarch64-linux-gnu"
		"powerpc64le-linux-gnu"
	)
else
	HOSTS="$@"
fi

wget https://github.com/seccomp/libseccomp/releases/download/v${VERSION}/libseccomp-${VERSION}.tar.gz
echo ${MD5SUM} libseccomp-${VERSION}.tar.gz | md5sum -c
tar xf libseccomp-${VERSION}.tar.gz && pushd libseccomp-${VERSION}

for host in "${HOSTS[@]}"; do
	./configure --host="${host}" --prefix=/usr --libdir=/usr/lib/"${host}"
	make -j$(nproc)
	make install
	make clean
	ldconfig /usr/lib/"${host}"
done

popd
rm -rf libseccomp-${VERSION}
