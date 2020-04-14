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
config install_weak_deps: False
update
install podman make golang-go libseccomp-devel bats jq
ts run
EOF
    dnf clean all
  SHELL
end
