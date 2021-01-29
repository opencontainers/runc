// +build linux

package fs

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

var FreezerTimeout = errors.New("set freezer status timeout")

type FreezerGroup struct {
}

func (s *FreezerGroup) Name() string {
	return "freezer"
}

func (s *FreezerGroup) Apply(path string, d *cgroupData) error {
	return join(path, d.pid)
}

func (s *FreezerGroup) Set(path string, cgroup *configs.Cgroup) error {
	switch cgroup.Resources.Freezer {
	case configs.Frozen, configs.Thawed:
		maxRetry := 5
		for {
			// In case this loop does not exit because it doesn't get the expected
			// state, let's write again this state, hoping it's going to be properly
			// set this time. Otherwise, this loop could run infinitely, waiting for
			// a state change that would never happen.
			if err := fscommon.WriteFile(path, "freezer.state", string(cgroup.Resources.Freezer)); err != nil {
				return err
			}

			var state configs.FreezerState
			var e error
			c := make(chan bool, 1)

			go func() {
				state, e = s.GetState(path)
				c <- true
			}()

			select {
			case <-c:
				if e != nil {
					return e
				}
				if state == cgroup.Resources.Freezer {
					return nil
				}
			case <-time.After(1 * time.Millisecond):
			}

			maxRetry = maxRetry - 1
			if maxRetry < 0 {
				return FreezerTimeout
			}

			time.Sleep(1 * time.Millisecond)
		}
	case configs.Undefined:
		return nil
	default:
		return fmt.Errorf("Invalid argument '%s' to freezer.state", string(cgroup.Resources.Freezer))
	}
}

func (s *FreezerGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

func (s *FreezerGroup) GetState(path string) (configs.FreezerState, error) {
	for {
		state, err := fscommon.ReadFile(path, "freezer.state")
		if err != nil {
			// If the kernel is too old, then we just treat the freezer as
			// being in an "undefined" state.
			if os.IsNotExist(err) || errors.Is(err, unix.ENODEV) {
				err = nil
			}
			return configs.Undefined, err
		}
		switch strings.TrimSpace(state) {
		case "THAWED":
			return configs.Thawed, nil
		case "FROZEN":
			return configs.Frozen, nil
		case "FREEZING":
			// Make sure we get a stable freezer state, so retry if the cgroup
			// is still undergoing freezing. This should be a temporary delay.
			time.Sleep(1 * time.Millisecond)
			continue
		default:
			return configs.Undefined, fmt.Errorf("unknown freezer.state %q", state)
		}
	}
}
