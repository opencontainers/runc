Name: runc
Version: %{_version}
Release: %{_release}%{?dist}
Summary: The open-source application container engine
Group: Tools/Runc

License: ASL 2.0
Source: %{name}.tar.gz

URL: https://opencontainers.org
Vendor: Open Containers Initiative
Packager: Open Containers Initiative <dev@opencontainers.org>

# docker builds in a checksum of dockerinit into docker,
# # so stripping the binaries breaks docker
%global __os_install_post %{_rpmconfigdir}/brp-compress
%global debug_package %{nil}

# required packages on install
Requires: /bin/sh
Requires: libcgroup
Requires: criu

%if 0%{?oraclelinux} == 6
# Require Oracle Unbreakable Enterprise Kernel R3 and newer device-mapper
Requires: kernel-uek >= 3.8
%endif

# docker-selinux conditional
%if 0%{?fedora} >= 20 || 0%{?centos} >= 7 || 0%{?rhel} >= 7 || 0%{?oraclelinux} >= 7
%global with_selinux 1
%endif

# start if with_selinux
%if 0%{?with_selinux}
# Version of SELinux we were using
%if 0%{?fedora} == 20
%global selinux_policyver 3.12.1-197
%endif # fedora 20
%if 0%{?fedora} == 21
%global selinux_policyver 3.13.1-105
%endif # fedora 21
%if 0%{?fedora} >= 22
%global selinux_policyver 3.13.1-128
%endif # fedora 22
%if 0%{?centos} >= 7 || 0%{?rhel} >= 7 || 0%{?oraclelinux} >= 7
%global selinux_policyver 3.13.1-23
%endif # centos,oraclelinux 7
%endif # with_selinux

# RE: rhbz#1195804 - ensure min NVR for selinux-policy
%if 0%{?with_selinux}
Requires: selinux-policy >= %{selinux_policyver}
%endif # with_selinux

%description
runc is a CLI tool for spawning and running containers according to the OCF 
specification.

%prep
%if 0%{?centos} <= 6
%setup -n %{name}
%else
%autosetup -n %{name}
%endif

%build
make all

%check
./runc -v

%install
# install binary
install -d $RPM_BUILD_ROOT/%{_bindir}
install -p -m 755 runc $RPM_BUILD_ROOT/%{_bindir}/runc

# list files owned by the package here
%files
/%{_bindir}/runc

%post


%preun


%postun


%changelog
