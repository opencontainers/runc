package fs

import (
	"strings"
	"time"

	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
)

type FreezerGroup struct {
}

func (s *FreezerGroup) Apply(d *data) error {
	switch d.c.Freezer {
	case configs.Frozen, configs.Thawed:
		dir, err := d.path("freezer")
		if err != nil {
			return err
		}

		if err := s.Set(dir, d.c); err != nil {
			return err
		}
	default:
		if _, err := d.join("freezer"); err != nil && !cgroups.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (s *FreezerGroup) Set(path string, cgroup *configs.Cgroup) error {
	switch cgroup.Freezer {
	case configs.Frozen, configs.Thawed:
		if err := writeFile(path, "freezer.state", string(cgroup.Freezer)); err != nil {
			return err
		}

		for {
			state, err := readFile(path, "freezer.state")
			if err != nil {
				return err
			}
			if strings.TrimSpace(state) == string(cgroup.Freezer) {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	return nil
}

func (s *FreezerGroup) Remove(d *data) error {
	return removePath(d.path("freezer"))
}

func (s *FreezerGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}
