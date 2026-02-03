ARG GO_VERSION=1.25
ARG BATS_VERSION=v1.12.0
ARG LIBSECCOMP_VERSION=2.6.0
ARG LIBPATHRS_VERSION=0.2.3

FROM golang:${GO_VERSION}-trixie
ARG DEBIAN_FRONTEND=noninteractive
ARG CRIU_REPO=https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_13

RUN KEYFILE=/usr/share/keyrings/criu-repo-keyring.gpg; \
    wget -nv $CRIU_REPO/Release.key -O- | gpg --dearmor > "$KEYFILE" \
    && echo "deb [signed-by=$KEYFILE] $CRIU_REPO/ /" > /etc/apt/sources.list.d/criu.list \
    && printf "%s\n" i386 armel armhf arm64 ppc64el s390x riscv64 | xargs -t -n1 -- dpkg --add-architecture \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        cargo \
        cargo-auditable \
        clang \
        criu \
        gcc \
        gcc-multilib \
        curl \
        gawk \
        gperf \
        iptables \
        jq \
        kmod \
        lld \
        pkg-config \
        python3-minimal \
        sshfs \
        sudo \
        uidmap \
        iproute2 \
    && apt-get install -y --no-install-recommends \
        libc-dev:i386 libgcc-s1:i386 gcc-i686-linux-gnu libstd-rust-dev:i386 \
        gcc-aarch64-linux-gnu libc-dev-arm64-cross libstd-rust-dev:arm64 \
        gcc-arm-linux-gnueabi libc-dev-armel-cross libstd-rust-dev:armel \
        gcc-arm-linux-gnueabihf libc-dev-armhf-cross libstd-rust-dev:armhf \
        gcc-powerpc64le-linux-gnu libc-dev-ppc64el-cross libstd-rust-dev:ppc64el \
        gcc-s390x-linux-gnu libc-dev-s390x-cross libstd-rust-dev:s390x \
        gcc-riscv64-linux-gnu libc-dev-riscv64-cross libstd-rust-dev:riscv64 \
    && apt-get clean \
    && rm -rf /var/cache/apt /var/lib/apt/lists/* /etc/apt/sources.list.d/*.list

# Add a dummy user for the rootless integration tests. While runC does
# not require an entry in /etc/passwd to operate, one of the tests uses
# `git clone` -- and `git clone` does not allow you to clone a
# repository if the current uid does not have an entry in /etc/passwd.
RUN useradd -u1000 -m -d/home/rootless -s/bin/bash rootless

# install bats
ARG BATS_VERSION
RUN cd /tmp \
    && git clone https://github.com/bats-core/bats-core.git \
    && cd bats-core \
    && git reset --hard "${BATS_VERSION}" \
    && ./install.sh /usr/local \
    && rm -rf /tmp/bats-core

ARG RELEASE_ARCHES="386 amd64 arm64 armel armhf ppc64le riscv64 s390x"
ENV DYLIB_DIR=/opt/runc-dylibs

# install libseccomp
ARG LIBSECCOMP_VERSION
COPY script/build-seccomp.sh script/lib.sh /tmp/script/
RUN mkdir -p $DYLIB_DIR \
    && /tmp/script/build-seccomp.sh "$LIBSECCOMP_VERSION" $DYLIB_DIR $RELEASE_ARCHES
ENV LIBSECCOMP_VERSION=$LIBSECCOMP_VERSION

# install libpathrs
ARG LIBPATHRS_VERSION
COPY script/build-libpathrs.sh /tmp/script/
RUN mkdir -p $DYLIB_DIR \
    && /tmp/script/build-libpathrs.sh "$LIBPATHRS_VERSION" $DYLIB_DIR $RELEASE_ARCHES
ENV LIBPATHRS_VERSION=$LIBPATHRS_VERSION

ENV LD_LIBRARY_PATH=$DYLIB_DIR/lib
ENV PKG_CONFIG_PATH=$DYLIB_DIR/lib/pkgconfig

# Prevent the "fatal: detected dubious ownership in repository" git complain during build.
RUN git config --global --add safe.directory /go/src/github.com/opencontainers/runc

WORKDIR /go/src/github.com/opencontainers/runc

# Fixup for cgroup v2.
COPY script/prepare-cgroup-v2.sh /
ENTRYPOINT [ "/prepare-cgroup-v2.sh" ]
