// build +linux

package main

import (
	"strings"
	"testing"

	"github.com/opencontainers/specs"
)

func TestLinuxCgroupsPathSpecified(t *testing.T) {
	cgroupsPath := "/user/cgroups/path/id"

	spec := &specs.LinuxSpec{}
	spec.Linux.CgroupsPath = &cgroupsPath

	cgroup, err := createCgroupConfig("ContainerID", spec)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	if cgroup.Path != cgroupsPath {
		t.Errorf("Wrong cgroupsPath, expected '%s' got '%s'", cgroupsPath, cgroup.Path)
	}
}

func TestLinuxCgroupsPathNotSpecified(t *testing.T) {
	spec := &specs.LinuxSpec{}

	cgroup, err := createCgroupConfig("ContainerID", spec)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	if !strings.HasSuffix(cgroup.Path, "/ContainerID") {
		t.Errorf("Wrong cgroupsPath, expected it to have suffix '%s' got '%s'", "/ContainerID", cgroup.Path)
	}
}
