//go:build linux && !runc_dmz_selinux_nocompat

package dmz

import (
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/selinux/go-selinux"
)

// WorksWithSELinux tells whether runc-dmz can work with SELinux.
//
// Older SELinux policy can prevent runc to execute the dmz binary. The issue is
// fixed in container-selinux >= 2.224.0:
//
//   - https://github.com/containers/container-selinux/issues/274
//   - https://github.com/containers/container-selinux/pull/280
//
// Alas, there is is no easy way to do a runtime check if dmz works with
// SELinux, so the below workaround is enabled by default. It results in
// disabling dmz in case container SELinux label is set and the selinux is in
// enforced mode.
//
// Newer distributions that have the sufficiently new container-selinux version
// can build runc with runc_dmz_selinux_nocompat build flag to disable this
// workaround (essentially allowing dmz to be used together with SELinux).
func WorksWithSELinux(c *configs.Config) bool {
	return c.ProcessLabel == "" || selinux.EnforceMode() != selinux.Enforcing
}
