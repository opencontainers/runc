#!/usr/bin/env bash
set -e -u

# bits of this were adapted from check_config.sh in docker
# see also https://github.com/docker/docker/blob/master/contrib/check-config.sh

possibleConfigs=(
	'/proc/config.gz'
	"/boot/config-$(uname -r)"
	"/usr/src/linux-$(uname -r)/.config"
	'/usr/src/linux/.config'
)
possibleConfigFiles=(
	'config.gz'
	"config-$(uname -r)"
	'.config'
)

if ! command -v zgrep &>/dev/null; then
	zgrep() {
		zcat "$2" | grep "$1"
	}
fi

kernelVersion="$(uname -r)"
kernelMajor="${kernelVersion%%.*}"
kernelMinor="${kernelVersion#"$kernelMajor".}"
kernelMinor="${kernelMinor%%.*}"

kernel_lt() {
	[ "$kernelMajor" -lt "$1" ] && return
	[ "$kernelMajor" -eq "$1" ] && [ "$kernelMinor" -le "$2" ]
}

is_set() {
	zgrep "CONFIG_$1=[y|m]" "$CONFIG" >/dev/null
}
is_set_in_kernel() {
	zgrep "CONFIG_$1=y" "$CONFIG" >/dev/null
}
is_set_as_module() {
	zgrep "CONFIG_$1=m" "$CONFIG" >/dev/null
}

color() {
	local codes=()
	if [ "$1" = 'bold' ]; then
		codes=("${codes[@]-}" '1')
		shift
	fi
	if [ "$#" -gt 0 ]; then
		local code=''
		case "$1" in
		# see https://en.wikipedia.org/wiki/ANSI_escape_code#Colors
		black) code=30 ;;
		red) code=31 ;;
		green) code=32 ;;
		yellow) code=33 ;;
		blue) code=34 ;;
		magenta) code=35 ;;
		cyan) code=36 ;;
		white) code=37 ;;
		esac
		if [ "$code" ]; then
			codes=("${codes[@]-}" "$code")
		fi
	fi
	local IFS=';'
	echo -en '\033['"${codes[*]-}"'m'
}
wrap_color() {
	text="$1"
	shift
	color "$@"
	echo -n "$text"
	color reset
	echo
}

wrap_good() {
	local name="$1"
	shift
	local val="$1"
	shift
	echo "$(wrap_color "$name" white): $(wrap_color "$val" green)" "$@"
}

wrap_bad() {
	local name="$1"
	shift
	local val="$1"
	shift
	echo "$(wrap_color "$name" bold): $(wrap_color "$val" bold red)" "$@"
}
wrap_warning() {
	wrap_color >&2 "$*" red
}

check_flag() {
	if is_set_in_kernel "$1"; then
		wrap_good "CONFIG_$1" 'enabled'
	elif is_set_as_module "$1"; then
		wrap_good "CONFIG_$1" 'enabled (as module)'
	else
		wrap_bad "CONFIG_$1" 'missing'
	fi
}

check_flags() {
	for flag in "$@"; do
		echo "- $(check_flag "$flag")"
	done
}

check_distro_userns() {
	[ -r /etc/os-release ] || return 0
	# shellcheck source=/dev/null
	. /etc/os-release 2>/dev/null || return 0
	if [[ "${ID}" =~ ^(centos|rhel)$ && "${VERSION_ID}" =~ ^7 ]]; then
		# this is a CentOS7 or RHEL7 system
		grep -q "user_namespace.enable=1" /proc/cmdline || {
			# no user namespace support enabled
			wrap_bad "  (RHEL7/CentOS7" "User namespaces disabled; add 'user_namespace.enable=1' to boot command line)"
		}
	fi
}

is_config() {
	local config="$1"

	# Todo: more check
	[[ -f "$config" ]] && return 0
	return 1
}

search_config() {
	local target_dir=("${1:-${possibleConfigs[@]}}")

	local tryConfig
	for tryConfig in "${target_dir[@]}"; do
		is_config "$tryConfig" && {
			CONFIG="$tryConfig"
			return
		}
		[[ -d "$tryConfig" ]] && {
			for tryFile in "${possibleConfigFiles[@]}"; do
				is_config "$tryConfig/$tryFile" && {
					CONFIG="$tryConfig/$tryFile"
					return
				}
			done
		}
	done

	wrap_warning "error: cannot find kernel config"
	wrap_warning "  try running this script again, specifying the kernel config:"
	wrap_warning "    CONFIG=/path/to/kernel/.config $0 or $0 /path/to/kernel/.config"
	exit 1
}

CONFIG="${1:-}"

is_config "$CONFIG" || {
	if [[ ! "$CONFIG" ]]; then
		wrap_color "info: no config specified, searching for kernel config ..." white
		search_config
	elif [[ -d "$CONFIG" ]]; then
		wrap_color "info: input is a directory, searching for kernel config in this directory..." white
		search_config "$CONFIG"
	else
		wrap_warning "warning: $CONFIG seems not a kernel config, searching other paths for kernel config ..."
		search_config
	fi
}

wrap_color "info: reading kernel config from $CONFIG ..." white
echo

echo 'Generally Necessary:'

cgroup=""
echo -n '- '
if [ "$(stat -f -c %t /sys/fs/cgroup 2>/dev/null)" = "63677270" ]; then
	wrap_good 'cgroup hierarchy' 'cgroupv2'
	cgroup="v2"
else
	cgroupSubsystemDir="$(awk '/[, ](cpu|cpuacct|cpuset|devices|freezer|memory)[, ]/ && $3 == "cgroup" { print $2 }' /proc/mounts | head -n1)"
	cgroupDir="$(dirname "$cgroupSubsystemDir")"
	if [ -d "$cgroupDir/cpu" ] || [ -d "$cgroupDir/cpuacct" ] || [ -d "$cgroupDir/cpuset" ] || [ -d "$cgroupDir/devices" ] || [ -d "$cgroupDir/freezer" ] || [ -d "$cgroupDir/memory" ]; then
		wrap_good 'cgroup hierarchy' 'properly mounted' "[$cgroupDir]"
		cgroup="v1"
	else
		if [ "$cgroupSubsystemDir" ]; then
			wrap_bad 'cgroup hierarchy' 'single mountpoint!' "[$cgroupSubsystemDir]"
		else
			wrap_bad 'cgroup hierarchy' 'nonexistent??'
		fi
		wrap_color '    (see https://github.com/tianon/cgroupfs-mount)' yellow
	fi
fi

if [ "$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null)" = 'Y' ]; then
	echo -n '- '
	if command -v apparmor_parser &>/dev/null; then
		wrap_good 'apparmor' 'enabled and tools installed'
	else
		wrap_bad 'apparmor' 'enabled, but apparmor_parser missing'
		echo -n '    '
		if command -v apt-get &>/dev/null; then
			wrap_color '(use "apt-get install apparmor" to fix this)' yellow
		elif command -v yum &>/dev/null; then
			wrap_color '(your best bet is "yum install apparmor-parser")' yellow
		else
			wrap_color '(look for an "apparmor" package for your distribution)' yellow
		fi
	fi
fi

flags=(
	NAMESPACES {NET,PID,IPC,UTS}_NS
	CGROUPS CGROUP_CPUACCT CGROUP_DEVICE CGROUP_FREEZER CGROUP_SCHED CPUSETS MEMCG
	KEYS
	VETH BRIDGE BRIDGE_NETFILTER
	IP_NF_FILTER IP_NF_TARGET_MASQUERADE
	NETFILTER_XT_MATCH_{ADDRTYPE,CONNTRACK,IPVS}
	IP_NF_NAT NF_NAT

	# required for bind-mounting /dev/mqueue into containers
	POSIX_MQUEUE
)
check_flags "${flags[@]}"

if ! kernel_lt 4 14; then
	if [ $cgroup = "v2" ]; then
		check_flags CGROUP_BPF
	fi
fi

if kernel_lt 5 1; then
	check_flags NF_NAT_IPV4
fi

if kernel_lt 5 2; then
	check_flags NF_NAT_NEEDED
fi

echo

echo 'Optional Features:'
{
	check_flags USER_NS
	check_distro_userns

	check_flags SECCOMP
	check_flags SECCOMP_FILTER
	check_flags CGROUP_PIDS

	check_flags MEMCG_SWAP

	if kernel_lt 5 8; then
		check_flags MEMCG_SWAP_ENABLED
		if is_set MEMCG_SWAP && ! is_set MEMCG_SWAP_ENABLED; then
			wrap_color '    (note that cgroup swap accounting is not enabled in your kernel config, you can enable it by setting boot option "swapaccount=1")' bold black
		fi
	fi
}

if kernel_lt 4 5; then
	check_flags MEMCG_KMEM
fi

if kernel_lt 3 18; then
	check_flags RESOURCE_COUNTERS
fi

if kernel_lt 3 13; then
	netprio=NETPRIO_CGROUP
else
	netprio=CGROUP_NET_PRIO
fi

if kernel_lt 5 0; then
	check_flags IOSCHED_CFQ CFQ_GROUP_IOSCHED
fi

flags=(
	BLK_CGROUP BLK_DEV_THROTTLING
	CGROUP_PERF
	CGROUP_HUGETLB
	NET_CLS_CGROUP "$netprio"
	CFS_BANDWIDTH FAIR_GROUP_SCHED RT_GROUP_SCHED
	IP_NF_TARGET_REDIRECT
	IP_VS
	IP_VS_NFCT
	IP_VS_PROTO_TCP
	IP_VS_PROTO_UDP
	IP_VS_RR
	SECURITY_SELINUX
	SECURITY_APPARMOR
)
check_flags "${flags[@]}"
