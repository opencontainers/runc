// +build linux,!no_systemd

package main

import (
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func shouldUseRootlessCgroupManagerUserns(context *cli.Context) (bool, error) {
	// [systemd driver]
	// We can call DetectUID() to parse the OwnerUID value from `busctl --user --no-pager status` result.
	// The value corresponds to sd_bus_creds_get_owner_uid(3).
	// If the value is 0, we have rootful systemd inside userns, so we do not need the rootless cgroup manager.
	//
	// On error, we assume we are root. An error may happen during shelling out to `busctl` CLI,
	// mostly when $DBUS_SESSION_BUS_ADDRESS is unset.
	if context.GlobalBool("systemd-cgroup") {
		ownerUID, err := systemd.DetectUID()
		if err != nil {
			logrus.WithError(err).Debug("failed to get the OwnerUID value, assuming the value to be 0")
			ownerUID = 0
		}
		return ownerUID != 0, nil
	}
	// [cgroupfs driver]
	// As we are unaware of cgroups path, we can't determine whether we have the full
	// access to the cgroups path.
	// Either way, we can safely decide to use the rootless cgroups manager.
	return true, nil
}
