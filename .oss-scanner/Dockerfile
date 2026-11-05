# Dockerfile for https://github.com/anthropics/oss-scanner.
#
# It builds runc and its test binaries, and fetches everything needed to run
# unit and integration tests, since the scan itself runs without network
# access. The only exception is the "spec validator" test in
# tests/integration/spec.bats, which clones runtime-spec and thus fails
# offline. See THREAT_MODEL.md for how to exercise runc.
#
# Keep versions in sync with the main Dockerfile.

ARG GO_VERSION=1.26
ARG BATS_VERSION=v1.12.0
ARG LIBPATHRS_VERSION=0.2.6

FROM golang:${GO_VERSION}-trixie
ARG DEBIAN_FRONTEND=noninteractive
ARG CRIU_REPO=https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_13

RUN KEYFILE=/usr/share/keyrings/criu-repo-keyring.gpg; \
    wget -nv $CRIU_REPO/Release.key -O- | gpg --dearmor > "$KEYFILE" \
    && echo "deb [signed-by=$KEYFILE] $CRIU_REPO/ /" > /etc/apt/sources.list.d/criu.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        cargo \
        criu \
        curl \
        gdb \
        iproute2 \
        iptables \
        jq \
        kmod \
        libseccomp-dev \
        lld \
        pkg-config \
        podman \
        python3-minimal \
        strace \
        sudo \
        uidmap \
    && apt-get clean \
    && rm -rf /var/cache/apt /var/lib/apt/lists/*

# A user for the rootless integration tests (see tests/rootless.sh).
RUN useradd -u1000 -m -d/home/rootless -s/bin/bash rootless

ARG BATS_VERSION
RUN cd /tmp \
    && git clone --depth 1 --branch "${BATS_VERSION}" https://github.com/bats-core/bats-core.git \
    && cd bats-core \
    && ./install.sh /usr \
    && rm -rf /tmp/bats-core

ENV DYLIB_DIR=/opt/runc-dylibs
ARG LIBPATHRS_VERSION
COPY script/build-libpathrs.sh script/lib.sh /tmp/script/
RUN mkdir -p $DYLIB_DIR \
    && /tmp/script/build-libpathrs.sh "$LIBPATHRS_VERSION" $DYLIB_DIR \
    && rm -rf /tmp/script ~/.cargo/registry \
    && echo "$DYLIB_DIR/lib" > /etc/ld.so.conf.d/libpathrs.conf \
    && ldconfig
# LD_LIBRARY_PATH is not enough, since runc init does not inherit it.
ENV PKG_CONFIG_PATH=$DYLIB_DIR/lib/pkgconfig

RUN git config --global --add safe.directory /src
COPY . /src
WORKDIR /src

# Allow "unsafe" integration tests in a container.
ENV RUNC_ALLOW_UNSAFE_TESTS=yes

# Build runc (with debug info) and the helper binaries used by the tests,
# and download the container images used by the integration tests.
RUN make runc test-binaries \
    && tests/integration/get-images.sh >/dev/null
