package cgroups

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/opencontainers/runc/internal/testutil"
)

func TestWriteCgroupFileHandlesInterrupt(t *testing.T) {
	testutil.SkipOnCentOS(t, "Flaky (see #3418)", 7)

	const (
		memoryCgroupMount = "/sys/fs/cgroup/memory"
		memoryLimit       = "memory.limit_in_bytes"
	)
	if _, err := os.Stat(memoryCgroupMount); err != nil {
		// most probably cgroupv2
		t.Skip(err)
	}

	cgroupName := fmt.Sprintf("test-eint-%d", time.Now().Nanosecond())
	cgroupPath := filepath.Join(memoryCgroupMount, cgroupName)
	if err := os.MkdirAll(cgroupPath, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cgroupPath)

	if _, err := os.Stat(filepath.Join(cgroupPath, memoryLimit)); err != nil {
		// either cgroupv2, or memory controller is not available
		t.Skip(err)
	}

	for i := 0; i < 100000; i++ {
		limit := 1024*1024 + i
		if err := WriteFile(cgroupPath, memoryLimit, strconv.Itoa(limit)); err != nil {
			t.Fatalf("Failed to write %d on attempt %d: %+v", limit, i, err)
		}
	}
}

func TestOpenat2(t *testing.T) {
	if !IsCgroup2UnifiedMode() {
		// The reason is many test cases below test opening files from
		// the top-level directory, where cgroup v1 has no files.
		t.Skip("test requires cgroup v2")
	}

	// Make sure we test openat2, not its fallback.
	openFallback = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
		return nil, errors.New("fallback")
	}
	defer func() { openFallback = openAndCheck }()

	for _, tc := range []struct{ dir, file string }{
		{"/sys/fs/cgroup", "cgroup.controllers"},
		{"/sys/fs/cgroup", "/cgroup.controllers"},
		{"/sys/fs/cgroup/", "cgroup.controllers"},
		{"/sys/fs/cgroup/", "/cgroup.controllers"},
		{"/", "/sys/fs/cgroup/cgroup.controllers"},
		{"/", "sys/fs/cgroup/cgroup.controllers"},
		{"/sys/fs/cgroup/cgroup.controllers", ""},
	} {
		fd, err := OpenFile(tc.dir, tc.file, os.O_RDONLY)
		if err != nil {
			t.Errorf("case %+v: %v", tc, err)
		}
		fd.Close()
	}
}
