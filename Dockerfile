ARG GO_VERSION=1.16
ARG BATS_VERSION=v1.3.0
ARG LIBSECCOMP_VERSION=2.5.2

FROM golang:${GO_VERSION}-buster
ARG DEBIAN_FRONTEND=noninteractive

RUN echo 'deb https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_10/ /' > /etc/apt/sources.list.d/criu.list \
    && wget -nv https://download.opensuse.org/repositories/devel:/tools:/criu/Debian_10/Release.key -O- | apt-key add - \
    && dpkg --add-architecture armel \
    && dpkg --add-architecture armhf \
    && dpkg --add-architecture arm64 \
    && dpkg --add-architecture ppc64el \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        criu \
        crossbuild-essential-arm64 \
        crossbuild-essential-armel \
        crossbuild-essential-armhf \
        crossbuild-essential-ppc64el \
        curl \
        gawk \
        gcc \
        gperf \
        iptables \
        jq \
        kmod \
        pkg-config \
        python-minimal \
        sshfs \
        sudo \
        uidmap \
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

# install libseccomp
ARG LIBSECCOMP_VERSION
COPY script/* /tmp/script/
RUN mkdir -p /opt/libseccomp \
    && /tmp/script/seccomp.sh "$LIBSECCOMP_VERSION" /opt/libseccomp arm64 armel armhf ppc64le
ENV LIBSECCOMP_VERSION=$LIBSECCOMP_VERSION
ENV LD_LIBRARY_PATH=/opt/libseccomp/lib
ENV PKG_CONFIG_PATH=/opt/libseccomp/lib/pkgconfig

WORKDIR /go/src/github.com/opencontainers/runc
