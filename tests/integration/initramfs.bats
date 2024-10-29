#!/usr/bin/env bats

load helpers

# Rather than building our own kernel for use with qemu, just reuse the host's
# kernel since we just need some kernel that supports containers that we can
# use to run our custom initramfs.
function find_vmlinuz() {
	shopt -s nullglob
	local candidate candidates=(
		/boot/vmlinuz
		/boot/vmlinuz-"$(uname -r)"*
		/usr/lib*/modules/"$(uname -r)"/vmlinuz*
	)
	shopt -u nullglob

	for candidate in "${candidates[@]}"; do
		[ -e "$candidate" ] || continue
		export HOST_KERNEL="$candidate"
		return 0
	done

	# Actuated doesn't provide a copy of the boot kernel, so we have to skip
	# the test in that case. It also seems they don't allow aarch64 guests
	# either (see <https://docs.actuated.com/examples/kvm-guest/>).
	skip "could not find host vmlinuz kernel"
}

function setup() {
	INITRAMFS_ROOT="$(mktemp -d "$BATS_RUN_TMPDIR/runc-initramfs.XXXXXX")"
	find_vmlinuz
}

function teardown() {
	[ -v INITRAMFS_ROOT ] && rm -rf "$INITRAMFS_ROOT"
}

function qemu_native() {
	# Different distributions put qemu-kvm in different locations and with
	# different names. Debian and Ubuntu have a "kvm" binary, while AlmaLinux
	# has /usr/libexec/qemu-kvm.
	local qemu_binary="" qemu_candidates=("kvm" "qemu-kvm" "/usr/libexec/qemu-kvm")
	local candidate
	for candidate in "${qemu_candidates[@]}"; do
		"$candidate" -help &>/dev/null || continue
		qemu_binary="$candidate"
		break
	done
	# TODO: Maybe we should also try to call qemu-system-FOO for the current
	# architecture if qemu-kvm is missing?
	[ -n "$qemu_binary" ] || skip "could not find qemu-kvm binary"

	local machine=
	case "$(go env GOARCH)" in
	386 | amd64)
		# Try to use a slightly newer PC CPU.
		machine="pc"
		;;
	arm | arm64)
		# ARM doesn't provide a "default" machine value (because its use is so
		# varied) so we have to specify the machine manually.
		machine="virt"
		;;
	*)
		echo "could not figure out -machine argument for qemu -- using default" >&2
		;;
	esac
	# We use -cpu max to ensure that the glibc we built runc with doesn't rely
	# on CPU features that the default QEMU CPU doesn't support (such as on
	# AlmaLinux 9).
	local machine_args=("-cpu" "max")
	[ -n "$machine" ] && machine_args+=("-machine" "$machine")

	sane_run --timeout=3m \
		"$qemu_binary" "${machine_args[@]}" "$@"
	if [ "$status" -ne 0 ]; then
		# To help with debugging, output the set of valid machine values.
		"$qemu_binary" -machine help >&2
	fi
}

@test "runc run [initramfs + pivot_root]" {
	requires root

	# Configure our minimal initrd.
	mkdir -p "$INITRAMFS_ROOT/initrd"
	pushd "$INITRAMFS_ROOT/initrd"

	# Use busybox as a base for our initrd.
	tar --exclude './dev/*' -xf "$BUSYBOX_IMAGE"
	# Make sure that "sh" and "poweroff" are installed, otherwise qemu will
	# boot loop when init stops.
	[ -x ./bin/sh ] || skip "busybox image is missing /bin/sh"
	[ -x ./bin/poweroff ] || skip "busybox image is missing /bin/poweroff"

	# Copy the runc binary into the container. In theory we would prefer to
	# copy a static binary, but some distros (like openSUSE) don't ship
	# libseccomp-static so requiring a static build for any integration test
	# run wouldn't work. Instead, we copy all of the library dependencies into
	# the rootfs (note that we also have to copy ld-linux-*.so because runc was
	# probably built with a newer glibc than the one in our busybox image.
	cp "$RUNC" ./bin/runc
	readarray -t runclibs \
		<<<"$(ldd "$RUNC" | grep -Eo '/[^ ]*lib[^ ]*.so.[^ ]*')"
	cp -vt ./lib64/ "${runclibs[@]}"
	# busybox has /lib64 -> /lib so we can just fill in one path.

	# Create a container bundle using the same busybox image.
	mkdir -p ./run/bundle
	pushd ./run/bundle
	mkdir -p rootfs
	tar --exclude './dev/*' -C rootfs -xf "$BUSYBOX_IMAGE"
	runc spec
	update_config '.process.args = ["/bin/echo", "hello from inside the container"]'
	popd

	# Build a custom /init script.
	cat >./init <<-EOF
		#!/bin/sh

		set -x
		echo "==START INIT SCRIPT=="

		mkdir -p /proc
		mount -t proc proc /proc
		mkdir -p /sys
		mount -t sysfs sysfs /sys

		mkdir -p /sys/fs/cgroup
		mount -t cgroup2 cgroup2 /sys/fs/cgroup

		mkdir -p /tmp
		mount -t tmpfs tmpfs /tmp

		mkdir -p /dev
		mount -t devtmpfs devtmpfs /dev
		mkdir -p /dev/pts
		mount -t devpts -o newinstance devpts /dev/pts
		mkdir -p /dev/shm
		mount --bind /tmp /dev/shm

		# Wait for as little as possible if we panic so we can output the error
		# log as part of the test failure before the test times out.
		echo 1 >/proc/sys/kernel/panic

		runc run -b /run/bundle ctr

		echo "==END INIT SCRIPT=="
		poweroff -f
	EOF
	chmod +x ./init

	find . | cpio -o -H newc >"$INITRAMFS_ROOT/initrd.cpio"
	popd

	# Now we can just run the image (use qemu-kvm so that we run on the same
	# architecture as the host system). We can just reuse the host kernel.
	qemu_native \
		-initrd "$INITRAMFS_ROOT/initrd.cpio" \
		-kernel "$HOST_KERNEL" \
		-m 512M \
		-nographic -append console=ttyS0 -no-reboot
	[ "$status" -eq 0 ]
	[[ "$output" = *"==START INIT SCRIPT=="* ]]
	[[ "$output" = *"hello from inside the container"* ]]
	[[ "$output" = *"==END INIT SCRIPT=="* ]]
}
