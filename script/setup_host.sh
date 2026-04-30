#!/bin/bash
# This script is used for initializing the host environment for CI.
# Supports Fedora and EL-based distributions.
set -eux -o pipefail

: "${LIBPATHRS_VERSION:=0.2.4}"

# BATS_VERSION is only consumed for the EL8 platform as its bats package is too old.
: "${BATS_VERSION:=v1.12.0}"

SCRIPTDIR="$(dirname "${BASH_SOURCE[0]}")"

# PLATFORM_ID is not available on Fedora
PLATFORM_ID=
grep -q ^PLATFORM_ID /etc/os-release && PLATFORM_ID="$(grep -oP '^PLATFORM_ID="\K[^"]+' /etc/os-release)"

# Initialize DNF
DNF=(dnf -y --setopt=install_weak_deps=False --setopt=tsflags=nodocs)
case "$PLATFORM_ID" in
platform:el8)
	# DNF+=(--exclude="kernel,kernel-core") seems to fail
	"${DNF[@]}" config-manager --set-enabled powertools # for glibc-static
	"${DNF[@]}" install epel-release
	;;
platform:el9 | platform:el10)
	DNF+=(--exclude="kernel,kernel-core")
	"${DNF[@]}" config-manager --set-enabled crb # for glibc-static
	"${DNF[@]}" install epel-release
	;;
*)
	# Fedora
	DNF5=yes
	DNF+=(--exclude="kernel,kernel-core")
	;;
esac

# Install common packages
RPMS=(cargo container-selinux fuse-sshfs git-core glibc-static golang iptables jq libseccomp-devel lld make policycoreutils wget)
# Work around dnf mirror failures by retrying a few times.
for i in $(seq 0 2); do
	sleep "$i"
	# Install and upgrade in a single transaction using dnf do or dnf shell.
	if [ -v DNF5 ]; then
		# dnf5: use dnf do.
		"${DNF[@]}" 'do' --action upgrade '*' --action=install "${RPMS[@]}" && break
	else
		# dnf4: use dnf shell.
		cat << _EOF_ | "${DNF[@]}" shell && break
update
install ${RPMS[@]}
ts run
_EOF_
	fi
done
# shellcheck disable=SC2181
[ $? -eq 0 ] # fail if dnf failed

# Install CRIU
if [ "$PLATFORM_ID" = "platform:el8" ]; then
	# Use newer criu (with https://github.com/checkpoint-restore/criu/pull/2545).
	# Alas we have to disable container-tools for that.
	"${DNF[@]}" module disable container-tools
	"${DNF[@]}" copr enable adrian/criu-el8
fi
"${DNF[@]}" install criu

# Install BATS
if [ "$PLATFORM_ID" = "platform:el8" ]; then
	# The packaged version of bats is too old: `BATS_ERROR_SUFFIX: unbound variable`, `bats_require_minimum_version: command not found`
	(
		cd /tmp
		git clone https://github.com/bats-core/bats-core
		(
			cd bats-core
			git checkout "$BATS_VERSION"
			./install.sh /usr/local
			cat >>/etc/profile.d/sh.local <<'EOF'
PATH="/usr/local/bin:$PATH"
export PATH
EOF
			cat >/etc/sudoers.d/local <<'EOF'
Defaults    secure_path = "/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin"
EOF
		)
		rm -rf bats-core
	)
else
	"${DNF[@]}" install bats
fi

# Clean up DNF
dnf clean all

# Install libpathrs
"$SCRIPTDIR"/build-libpathrs.sh "$LIBPATHRS_VERSION" /usr

# Setup rootless user.
"$SCRIPTDIR"/setup_rootless.sh

# Delegate all cgroup v2 controllers to rootless user via --systemd-cgroup
if [ -e /sys/fs/cgroup/cgroup.controllers ]; then
	mkdir -p /etc/systemd/system/user@.service.d
	cat >/etc/systemd/system/user@.service.d/delegate.conf <<'EOF'
[Service]
# The default (since systemd v252) is "pids memory cpu".
Delegate=yes
EOF
	systemctl daemon-reload
fi

# Allow potentially unsafe tests.
echo 'export RUNC_ALLOW_UNSAFE_TESTS=yes' >>/root/.bashrc
