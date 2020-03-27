// +build linux

package fscommon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func TestWriteCgroupFileHandlesInterrupt(t *testing.T) {
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skip("cgroup v2 is not supported")
	}

	memoryCgroupMount, err := cgroups.FindCgroupMountpoint("", "memory")
	if err != nil {
		t.Fatal(err)
	}

	cgroupName := fmt.Sprintf("test-eint-%d", time.Now().Nanosecond())
	cgroupPath := filepath.Join(memoryCgroupMount, cgroupName)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cgroupPath)

	for i := 0; i < 100000; i++ {
		limit := 1024*1024 + i
		if err := WriteFile(cgroupPath, "memory.limit_in_bytes", strconv.Itoa(limit)); err != nil {
			t.Fatalf("Failed to write %d on attempt %d: %+v", limit, i, err)
		}
	}
}
