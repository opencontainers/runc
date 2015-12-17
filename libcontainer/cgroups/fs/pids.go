// +build linux

package fs

import (
	"fmt"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"strconv"
)

type PidsGroup struct {
}

func (s *PidsGroup) Name() string {
	return "pids"
}

func (s *PidsGroup) Apply(d *data) error {
	dir, err := d.join("pids")
	if err != nil {
		// since Linux 4.3
		if cgroups.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := s.Set(dir, d.c); err != nil {
		return err
	}

	return nil
}

func (s *PidsGroup) Set(path string, cgroup *configs.Cgroup) error {

	if cgroup.PidsMax > 0 {
		if err := writeFile(path, "pids.max", fmt.Sprint(cgroup.PidsMax)); err != nil {
			return err
		}
	} else if cgroup.PidsMax < 0 {
		if err := writeFile(path, "pids.max", "max"); err != nil {
			return err
		}
	}

	for pid := range cgroup.Pids {
		if err := writeFile(path, "cgroup.procs", strconv.Itoa(pid)); err != nil {
			return err
		}
	}

	return nil
}

func (s *PidsGroup) Remove(d *data) error {
	return removePath(d.path("pids"))
}

func (s *PidsGroup) GetStats(path string, stats *cgroups.Stats) error {

	values, err := getCgroupParamUintArray(path, "pids.current")
	if err != nil {
		return fmt.Errorf("failed to parse pids.current - %v", err)
	}

	stats.PidsStats.Current = values

	value, err := getCgroupParamString(path, "pids.max")
	if err != nil {
		return fmt.Errorf("failed to parse pids.max - %v", err)
	}

	if "max" == value {
		stats.PidsStats.Max = configs.MaxPids
	} else {
		max, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("failed to parse pids.max content %s as int", value)
		}
		stats.PidsStats.Max = max
	}

	return nil
}
