// +build linux

package fscommon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestWriteCgroupFileHandlesInterrupt(t *testing.T) {
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
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
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
