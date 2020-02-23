ARG GO_VERSION=1.12.17
ARG BATS_VERSION=03608115df2071fff4eaaff1605768c275e5f81f
ARG CRIU_VERSION=v3.12
ARG DEBIAN_FRONTEND=noninteractive

FROM golang:${GO_VERSION}-buster
ARG DEBIAN_FRONTEND

RUN dpkg --add-architecture armel \
    && dpkg --add-architecture armhf \
    && dpkg --add-architecture arm64 \
    && dpkg --add-architecture ppc64el

RUN apt-get update && apt-get install -y --no-install-recommends \
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
        pkg-config \
        protobuf-c-compiler \
        protobuf-compiler \
        python-minimal \
        sudo \
        uidmap \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*;

# Add a dummy user for the rootless integration tests. While runC does
# not require an entry in /etc/passwd to operate, one of the tests uses
# `git clone` -- and `git clone` does not allow you to clone a
# repository if the current uid does not have an entry in /etc/passwd.
RUN useradd -u1000 -m -d/home/rootless -s/bin/bash rootless

# install bats
ARG BATS_VERSION
RUN cd /tmp \
    && git clone https://github.com/sstephenson/bats.git \
    && cd bats \
    && git reset --hard "${BATS_VERSION}" \
    && ./install.sh /usr/local \
    && rm -rf /tmp/bats

# install criu
ARG CRIU_VERSION
RUN mkdir -p /usr/src/criu \
    && curl -sSL https://github.com/checkpoint-restore/criu/archive/${CRIU_VERSION}.tar.gz | tar -v -C /usr/src/criu/ -xz --strip-components=1 \
    && cd /usr/src/criu \
    && make install-criu \
    && rm -rf /usr/src/criu

COPY script/tmpmount /
WORKDIR /go/src/github.com/opencontainers/runc
ENTRYPOINT ["/tmpmount"]

# setup a playground for us to spawn containers in
COPY tests/integration/multi-arch.bash tests/integration/
ENV ROOTFS /busybox
RUN mkdir -p ${ROOTFS}
RUN . tests/integration/multi-arch.bash \
    && curl -o- -sSL `get_busybox` | tar xfJC - ${ROOTFS}

COPY . .
