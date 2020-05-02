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
    curl -OSs https://kojipkgs.fedoraproject.org/packages/criu/3.14/1.fc32/x86_64/criu-3.14-1.fc32.x86_64.rpm
    cat << EOF | dnf -y shell
localinstall criu-3.14-1.fc32.x86_64.rpm
update
install iptables gcc make golang-go libseccomp-devel bats jq
ts run
EOF
    dnf clean all
    rm -f criu-3.14-1.fc32.x86_64.rpm

    # Add a user for rootless tests
    useradd -u2000 -m -d/home/rootless -s/bin/bash rootless

    # Add busybox for libcontainer/integration tests
    . /vagrant/tests/integration/multi-arch.bash \
        && mkdir /busybox \
        && curl -fsSL $(get_busybox) | tar xfJC - /busybox
  SHELL
end
