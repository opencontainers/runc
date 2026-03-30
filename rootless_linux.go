package main

import (
	"os"

	"github.com/moby/sys/userns"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/opencontainers/cgroups/systemd"
)

func shouldUseRootlessCgroupManager(cmd *cli.Command) (bool, error) {
	if cmd != nil {
		b, err := parseBoolOrAuto(cmd.String("rootless"))
		if err != nil {
			return false, err
		}
		// nil b stands for "auto detect"
		if b != nil {
			return *b, nil
		}
	}
	if os.Geteuid() != 0 {
		return true, nil
	}
	if !userns.RunningInUserNS() {
		// euid == 0 , in the initial ns (i.e. the real root)
		return false, nil
	}
	// euid = 0, in a userns.
	//
	// [systemd driver]
	// We can call DetectUID() to parse the OwnerUID value from `busctl --user --no-pager status` result.
	// The value corresponds to sd_bus_creds_get_owner_uid(3).
	// If the value is 0, we have rootful systemd inside userns, so we do not need the rootless cgroup manager.
	//
	// On error, we assume we are root. An error may happen during shelling out to `busctl` CLI,
	// mostly when $DBUS_SESSION_BUS_ADDRESS is unset.
	if cmd.Bool("systemd-cgroup") {
		ownerUID, err := systemd.DetectUID()
		if err != nil {
			logrus.WithError(err).Debug("failed to get the OwnerUID value, assuming the value to be 0")
			ownerUID = 0
		}
		return ownerUID != 0, nil
	}
	// [cgroupfs driver]
	// As we are unaware of cgroups path, we can't determine whether we have to full
	// access to the cgroups path.
	// Either way, we can safely decide to use the rootless cgroups manager.
	return true, nil
}

func shouldHonorXDGRuntimeDir() bool {
	if os.Geteuid() != 0 {
		return true
	}
	if !userns.RunningInUserNS() {
		// euid == 0 , in the initial ns (i.e. the real root)
		// in this case, we should use /run/runc and ignore
		// $XDG_RUNTIME_DIR (e.g. /run/user/0) for backward
		// compatibility.
		return false
	}
	// euid = 0, in a userns.
	u, ok := os.LookupEnv("USER")
	return !ok || u != "root"
}
