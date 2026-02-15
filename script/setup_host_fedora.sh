#!/bin/bash
set -eux -o pipefail
DNF=(dnf -y --setopt=install_weak_deps=False --setopt=tsflags=nodocs --exclude="kernel,kernel-core")
RPMS=(bats git-core glibc-static golang jq libseccomp-devel cargo lld make)
# Work around dnf mirror failures by retrying a few times.
for i in $(seq 0 2); do
	sleep "$i"
	"${DNF[@]}" update && "${DNF[@]}" install "${RPMS[@]}" && break
done

# criu-4.1-1 has a known bug (https://github.com/checkpoint-restore/criu/issues/2650)
# which is fixed in criu-4.1-2 (currently in updates-testing). TODO: remove this later.
if [[ $(rpm -q criu) == "criu-4.1-1.fc"* ]]; then
	"${DNF[@]}" --enablerepo=updates-testing update criu
fi

dnf clean all

SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"

LIBPATHRS_VERSION=0.2.3
"$SCRIPTDIR"/build-libpathrs.sh "$LIBPATHRS_VERSION" /usr

# To avoid "avc: denied { nosuid_transition }" from SELinux as we run tests on /tmp.
mount -o remount,suid /tmp

# Setup rootless user.
"$SCRIPTDIR"/setup_rootless.sh

# Delegate cgroup v2 controllers to rootless user via --systemd-cgroup
mkdir -p /etc/systemd/system/user@.service.d
cat >/etc/systemd/system/user@.service.d/delegate.conf <<EOF
[Service]
# default: Delegate=pids memory
# NOTE: delegation of cpuset requires systemd >= 244 (Fedora >= 32, Ubuntu >= 20.04).
Delegate=yes
EOF
systemctl daemon-reload
