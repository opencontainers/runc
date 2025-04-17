#!/bin/bash
set -eux -o pipefail
DNF=(dnf -y --setopt=install_weak_deps=False --setopt=tsflags=nodocs --exclude="kernel,kernel-core")
RPMS=(bats git-core glibc-static golang jq libseccomp-devel make)
# Work around dnf mirror failures by retrying a few times.
for i in $(seq 0 2); do
	sleep "$i"
	"${DNF[@]}" update && "${DNF[@]}" install "${RPMS[@]}" && break
done
dnf clean all

# To avoid "avc: denied { nosuid_transition }" from SELinux as we run tests on /tmp.
mount -o remount,suid /tmp

# Setup rootless user.
"$(dirname "${BASH_SOURCE[0]}")"/setup_rootless.sh

# Delegate cgroup v2 controllers to rootless user via --systemd-cgroup
mkdir -p /etc/systemd/system/user@.service.d
cat >/etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
# default: Delegate=pids memory
# NOTE: delegation of cpuset requires systemd >= 244 (Fedora >= 32, Ubuntu >= 20.04).
Delegate=yes
EOF
systemctl daemon-reload
