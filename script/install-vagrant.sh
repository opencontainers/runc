#!/bin/bash
set -eux -o pipefail
VAGRANT_VERSION="2.2.7"

# https://github.com/alvistack/ansible-role-virtualbox/blob/6887b020b0ca5c59ddb6620d73f053ffb84f4126/.travis.yml#L30
apt-get update
apt-get install -q -y bridge-utils dnsmasq-base ebtables libvirt-bin libvirt-dev qemu-kvm qemu-utils ruby-dev
wget https://releases.hashicorp.com/vagrant/${VAGRANT_VERSION}/vagrant_${VAGRANT_VERSION}_$(uname -m).deb
dpkg -i vagrant_${VAGRANT_VERSION}_$(uname -m).deb
rm -f vagrant_${VAGRANT_VERSION}_$(uname -m).deb
vagrant plugin install vagrant-libvirt
