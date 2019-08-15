// +build linux

package fs

import (
	"fmt"
	"strings"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type FreezerGroup struct {
}

func (s *FreezerGroup) Name() string {
	return "freezer"
}

func (s *FreezerGroup) Apply(d *cgroupData) error {
	_, err := d.join("freezer")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *FreezerGroup) RecursiveThaw(path string) error {
	return cgroups.WalkCgroups(path, func(dir string) error {
		return s.set(dir, configs.Thawed)
	})
}

func (s *FreezerGroup) Remove(d *cgroupData) error {
	return removePath(d.path("freezer"))
}

func (s *FreezerGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

func (s *FreezerGroup) Set(path string, cgroup *configs.Cgroup) error {
	return s.set(path, cgroup.Resources.Freezer)
}

func (s *FreezerGroup) set(path string, state configs.FreezerState) error {
	switch state {
	case configs.Frozen, configs.Thawed:
		for {
			// In case this loop does not exit because it doesn't get the expected
			// state, let's write again this state, hoping it's going to be properly
			// set this time. Otherwise, this loop could run infinitely, waiting for
			// a state change that would never happen.
			if err := writeFile(path, "freezer.state", string(state)); err != nil {
				return err
			}

			current, err := readFile(path, "freezer.state")
			if err != nil {
				return err
			}
			if strings.TrimSpace(current) == string(state) {
				break
			}

			time.Sleep(1 * time.Millisecond)
		}
	case configs.Undefined:
		return nil
	default:
		return fmt.Errorf("Invalid argument '%s' to freezer.state", string(state))
	}
	return nil
}
