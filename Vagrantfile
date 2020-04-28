# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
# Fedora box is used for testing cgroup v2 support
  config.vm.box = "fedora/32-cloud-base"
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
install iptables gcc make golang-go libseccomp-devel bats jq \
 patch protobuf protobuf-c protobuf-c-compiler protobuf-c-devel protobuf-compiler \
 protobuf-devel libnl3-devel libcap-devel libnet-devel \
 nftables-devel libbsd-devel gnutls-devel
ts run
EOF
    dnf clean all

    # Add a user for rootless tests
    useradd -u2000 -m -d/home/rootless -s/bin/bash rootless

    # Add busybox for libcontainer/integration tests
    . /vagrant/tests/integration/multi-arch.bash \
        && mkdir /busybox \
        && curl -fsSL $(get_busybox) | tar xfJC - /busybox

    # Apr 25, 2020 (master)
    ( git clone https://github.com/checkpoint-restore/criu.git /usr/src/criu \
     && cd /usr/src/criu \
     && git checkout 5c5e7695a51318b17e3d982df8231ac83971641c  \
     && make install-criu )
    rm -rf /usr/src/criu
  SHELL
end
