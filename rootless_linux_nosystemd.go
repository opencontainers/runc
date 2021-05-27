// +build linux,no_systemd

package main

import "github.com/urfave/cli"

func shouldUseRootlessCgroupManagerUserns(context *cli.Context) (bool, error) {
	// [cgroupfs driver]
	// As we are unaware of cgroups path, we can't determine whether we have the full
	// access to the cgroups path.
	// Either way, we can safely decide to use the rootless cgroups manager.
	return true, nil
}
