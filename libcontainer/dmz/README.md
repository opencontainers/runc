# Runc-dmz

runc-dmz is a small and very simple binary used to execute the container's entrypoint.

## Making it small

To make it small we use the Linux kernel's [nolibc include files][nolibc-upstream], so we don't use the libc.

A full `cp` of it is here in `nolibc/`, but removing the Makefile that is GPL. DO NOT FORGET to
remove the GPL code if updating the nolibc/ directory.

The current version in that folder is from Linux 6.6-rc3 tag (556fb7131e03b0283672fb40f6dc2d151752aaa7).

It also support all the architectures we support in runc.

If the GOARCH we use for compiling doesn't support nolibc, it fallbacks to using the C stdlib.

## SELinux compatibility issue and a workaround

Older SELinux policy can prevent runc to execute the dmz binary. The issue is
fixed in [container-selinux v2.224.0]. Yet, some older distributions may not
have the fix, so runc has a runtime workaround of disabling dmz if it finds
that SELinux is in enforced mode and the container SELinux label is set.

Distributions that have a sufficiently new container-selinux can disable the
workaround by building runc with the `runc_dmz_selinux_nocompat` build flag,
essentially allowing dmz to be used together with SELinux.

[nolibc-upstream]: https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/tools/include/nolibc?h=v6.6-rc3
[container-selinux v2.224.0]: https://github.com/containers/container-selinux/releases/tag/v2.224.0
