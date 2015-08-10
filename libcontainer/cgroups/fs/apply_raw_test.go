package fs

import (
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func TestCgroupParent(t *testing.T) {
	raw := &data{
		cgroup: "",
	}
	subsystem := "memory"
	subPath := "/docker/874e38c82a6c630dc95f3f1f2c8d8f43efb531d35a9f46154ab2fde1531b7bb6"
	initPath, err := cgroups.GetInitCgroupDir(subsystem)
	if err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(initPath, subPath)
	cgPath := "/sys/fs/cgroup/memory"
	path, err := raw.parent(subsystem, cgPath, srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(cgPath, subPath) {
		t.Fatalf("Unexpected path: %s, should be %s", path, filepath.Join(cgPath, subPath))
	}
}
