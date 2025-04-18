#!/bin/bash
set -eux -o pipefail
DNF=(dnf -y --setopt=install_weak_deps=False --setopt=tsflags=nodocs --exclude="kernel,kernel-core")
RPMS=(bats git-core glibc-static golang jq libseccomp-devel make)
# Work around dnf mirror failures by retrying a few times.
for i in $(seq 0 2); do
	sleep "$i"
	"${DNF[@]}" update && "${DNF[@]}" install "${RPMS[@]}" && break
done

# Bump criu to v4.1 for testing.
dnf -y --enablerepo=updates-testing update criu
dnf clean all

# To avoid "avc: denied { nosuid_transition }" from SELinux as we run tests on /tmp.
mount -o remount,suid /tmp

# Add a user for rootless tests
useradd -u2000 -m -d/home/rootless -s/bin/bash rootless

# Allow root and rootless itself to execute `ssh rootless@localhost` in tests/rootless.sh
ssh-keygen -t ecdsa -N "" -f /root/rootless.key
mkdir -m 0700 /home/rootless/.ssh
cp /root/rootless.key /home/rootless/.ssh/id_ecdsa
cat /root/rootless.key.pub >>/home/rootless/.ssh/authorized_keys
chown -R rootless.rootless /home/rootless

# Delegate cgroup v2 controllers to rootless user via --systemd-cgroup
mkdir -p /etc/systemd/system/user@.service.d
cat >/etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
# default: Delegate=pids memory
# NOTE: delegation of cpuset requires systemd >= 244 (Fedora >= 32, Ubuntu >= 20.04).
Delegate=yes
EOF
systemctl daemon-reload
