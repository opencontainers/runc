ARG GO_VERSION=1.26
ARG RUST_VERSION=1.99
ARG BATS_VERSION=v1.12.0
ARG LIBSECCOMP_VERSION=2.6.1
ARG LIBPATHRS_VERSION=0.2.6

FROM golang:${GO_VERSION}-trixie
ARG DEBIAN_FRONTEND=noninteractive
ARG CRIU_REPO=https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_13

RUN KEYFILE=/usr/share/keyrings/criu-repo-keyring.gpg; \
    wget -nv $CRIU_REPO/Release.key -O- | gpg --dearmor > "$KEYFILE" \
    && echo "deb [signed-by=$KEYFILE] $CRIU_REPO/ /" > /etc/apt/sources.list.d/criu.list \
    && printf "%s\n" armel armhf arm64 ppc64el riscv64 | xargs -t -n1 -- dpkg --add-architecture \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        cargo-auditable \
        clang \
        criu \
        gcc \
        curl \
        gawk \
        gperf \
        iptables \
        jq \
        kmod \
        lld \
        musl-dev \
        pkg-config \
        python3-minimal \
        rustup \
        sshfs \
        sudo \
        uidmap \
        iproute2 \
    && apt-get install -y --no-install-recommends \
        gcc-aarch64-linux-gnu libc-dev-arm64-cross musl-dev:arm64 \
        gcc-arm-linux-gnueabi libc-dev-armel-cross musl-dev:armel \
        gcc-arm-linux-gnueabihf libc-dev-armhf-cross musl-dev:armhf \
        gcc-powerpc64le-linux-gnu libc-dev-ppc64el-cross musl-dev:ppc64el \
        gcc-riscv64-linux-gnu libc-dev-riscv64-cross musl-dev:riscv64 \
        gcc-s390x-linux-gnu libc-dev-s390x-cross \
    && apt-get clean \
    && rm -rf /var/cache/apt /var/lib/apt/lists/* /etc/apt/sources.list.d/*.list

# Debian's musl-dev does not provide Linux kernel headers, so make the ones
# from linux-libc-dev available for musl builds (see set_cross_vars).
RUN for d in linux asm-generic x86_64-linux-gnu/asm; do \
        ln -s "/usr/include/$d" /usr/include/x86_64-linux-musl/; \
    done \
    && for t in aarch64-linux-gnu:aarch64-linux-musl \
            arm-linux-gnueabi:arm-linux-musleabi \
            arm-linux-gnueabihf:arm-linux-musleabihf \
            powerpc64le-linux-gnu:powerpc64le-linux-musl \
            riscv64-linux-gnu:riscv64-linux-musl; do \
        for d in linux asm asm-generic; do \
            ln -s "/usr/${t%:*}/include/$d" "/usr/include/${t#*:}/"; \
        done; \
    done

# Install Rust, with standard libraries for release targets (used by
# libpathrs build for release binaries, see build-libpathrs.sh).
ARG RUST_VERSION
RUN rustup toolchain install "$RUST_VERSION" --profile minimal --no-self-update \
        --target x86_64-unknown-linux-musl,aarch64-unknown-linux-musl,armv5te-unknown-linux-musleabi,armv7-unknown-linux-musleabihf,powerpc64le-unknown-linux-musl,riscv64gc-unknown-linux-musl,s390x-unknown-linux-gnu \
    && rustup default "$RUST_VERSION"

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

ARG RELEASE_ARCHES="amd64 arm64 armel armhf ppc64le riscv64 s390x"
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

# Allow "unsafe" integration tests in a container.
ENV RUNC_ALLOW_UNSAFE_TESTS=yes

# Fixup for cgroup v2.
COPY script/prepare-cgroup-v2.sh /
ENTRYPOINT [ "/prepare-cgroup-v2.sh" ]
