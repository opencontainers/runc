ARG GO_VERSION=1.13
ARG CRIU_VERSION=v3.13

FROM golang:${GO_VERSION}-buster
ARG DEBIAN_FRONTEND=noninteractive

RUN dpkg --add-architecture armel \
    && dpkg --add-architecture armhf \
    && dpkg --add-architecture arm64 \
    && dpkg --add-architecture ppc64el \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        crossbuild-essential-arm64 \
        crossbuild-essential-armel \
        crossbuild-essential-armhf \
        crossbuild-essential-ppc64el \
        curl \
        gawk \
        iptables \
        jq \
        kmod \
        libaio-dev \
        libcap-dev \
        libnet-dev \
        libnl-3-dev \
        libprotobuf-c-dev \
        libprotobuf-dev \
        libseccomp-dev \
        libseccomp-dev:arm64 \
        libseccomp-dev:armel \
        libseccomp-dev:armhf \
        libseccomp-dev:ppc64el \
        libseccomp2 \
        npm \
        pkg-config \
        protobuf-c-compiler \
        protobuf-compiler \
        python-minimal \
        sudo \
        uidmap \
    && apt-get clean \
    && rm -rf /var/cache/apt /var/lib/apt/lists/*;

# Add a dummy user for the rootless integration tests. While runC does
# not require an entry in /etc/passwd to operate, one of the tests uses
# `git clone` -- and `git clone` does not allow you to clone a
# repository if the current uid does not have an entry in /etc/passwd.
RUN useradd -u1000 -m -d/home/rootless -s/bin/bash rootless

# install bats
RUN npm install -g bats

# install criu
ARG CRIU_VERSION
RUN mkdir -p /usr/src/criu \
    && curl -fsSL https://github.com/checkpoint-restore/criu/archive/${CRIU_VERSION}.tar.gz | tar -C /usr/src/criu/ -xz --strip-components=1 \
    && cd /usr/src/criu \
    && echo 1 > .gitid \
    && curl -sSL https://github.com/checkpoint-restore/criu/commit/4c27b3db4f4325a311d8bfa9a50ea3efb4d6e377.patch | patch -p1 \
    && curl -sSL https://github.com/checkpoint-restore/criu/commit/aac41164b2cd7f0d2047f207b32844524682e43f.patch | patch -p1 \
    && curl -sSL https://github.com/checkpoint-restore/criu/commit/6f19249b2565f3f7c0a1f8f65b4ae180e8f7f34b.patch | patch -p1 \
    && curl -sSL https://github.com/checkpoint-restore/criu/commit/378337a496ca759848180bc5411e4446298c5e4e.patch | patch -p1 \
    && make install-criu \
    && cd - \
    && rm -rf /usr/src/criu

COPY script/tmpmount /
WORKDIR /go/src/github.com/opencontainers/runc
ENTRYPOINT ["/tmpmount"]

# setup a playground for us to spawn containers in
COPY tests/integration/multi-arch.bash tests/integration/
ENV ROOTFS /busybox
RUN mkdir -p "${ROOTFS}"
RUN . tests/integration/multi-arch.bash \
    && curl -fsSL `get_busybox` | tar xfJC - "${ROOTFS}"

COPY . .
