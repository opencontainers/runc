# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
# Fedora 31 box is used for testing cgroup v2 support
  config.vm.box = "fedora/31-cloud-base"
  config.vm.provider :virtualbox do |v|
    v.memory = 2048
    v.cpus = 2
  end
  config.vm.provider :libvirt do |v|
    v.memory = 2048
    v.cpus = 2
  end
  config.vm.provision "shell", inline: <<-SHELL
    cat << EOF | dnf -y shell
update
install podman make golang-go libseccomp-devel bats jq \
 patch protobuf protobuf-c protobuf-c-devel protobuf-compiler \
 protobuf-devel protobuf-python libnl3-devel libcap-devel libnet-devel \
 nftables-devel libbsd-devel gnutls-devel
ts run
EOF
    dnf clean all

    # TODO: remove this after criu 3.14 is released
    rpm -e --nodeps criu || true
    CRIU_VERSION=v3.13
    mkdir -p /usr/src/criu \
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
  SHELL
end
