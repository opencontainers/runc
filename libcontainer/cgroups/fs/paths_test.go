package fs

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestInvalidCgroupPath(t *testing.T) {
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skip("cgroup v2 is not supported")
	}

	root, err := rootPath()
	if err != nil {
		t.Fatalf("couldn't get cgroup root: %v", err)
	}

	testCases := []struct {
		test               string
		path, name, parent string
	}{
		{
			test: "invalid cgroup path",
			path: "../../../../../../../../../../some/path",
		},
		{
			test: "invalid absolute cgroup path",
			path: "/../../../../../../../../../../some/path",
		},
		{
			test:   "invalid cgroup parent",
			parent: "../../../../../../../../../../some/path",
			name:   "name",
		},
		{
			test:   "invalid absolute cgroup parent",
			parent: "/../../../../../../../../../../some/path",
			name:   "name",
		},
		{
			test:   "invalid cgroup name",
			parent: "parent",
			name:   "../../../../../../../../../../some/path",
		},
		{
			test:   "invalid absolute cgroup name",
			parent: "parent",
			name:   "/../../../../../../../../../../some/path",
		},
		{
			test:   "invalid cgroup name and parent",
			parent: "../../../../../../../../../../some/path",
			name:   "../../../../../../../../../../some/path",
		},
		{
			test:   "invalid absolute cgroup name and parent",
			parent: "/../../../../../../../../../../some/path",
			name:   "/../../../../../../../../../../some/path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			config := &configs.Cgroup{Path: tc.path, Name: tc.name, Parent: tc.parent}

			inner, err := innerPath(config)
			if err != nil {
				t.Fatalf("couldn't get cgroup data: %v", err)
			}

			// Make sure the final inner path doesn't go outside the cgroup mountpoint.
			if strings.HasPrefix(inner, "..") {
				t.Errorf("SECURITY: cgroup innerPath is outside cgroup mountpoint!")
			}

			// Double-check, using an actual cgroup.
			deviceRoot := filepath.Join(root, "devices")
			devicePath, err := subsysPath(root, inner, "devices")
			if err != nil {
				t.Fatalf("couldn't get cgroup path: %v", err)
			}
			if !strings.HasPrefix(devicePath, deviceRoot) {
				t.Errorf("SECURITY: cgroup path() is outside cgroup mountpoint!")
			}
		})
	}
}

func TestTryDefaultCgroupRoot(t *testing.T) {
	res := tryDefaultCgroupRoot()
	exp := defaultCgroupRoot
	if cgroups.IsCgroup2UnifiedMode() {
		// checking that tryDefaultCgroupRoot does return ""
		// in case /sys/fs/cgroup is not cgroup v1 root dir.
		exp = ""
	}
	if res != exp {
		t.Errorf("tryDefaultCgroupRoot: want %q, got %q", exp, res)
	}
}
