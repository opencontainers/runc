package userns

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

// startUserns starts a process in a new user namespace, returning its pid.
func startUserns(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sleep", "infinity")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Geteuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getegid(), Size: 1}},
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("can't create user namespace: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd.Process.Pid
}

func TestGetUserNamespaceMappings(t *testing.T) {
	pid := strconv.Itoa(startUserns(t))
	want := []configs.IDMap{{ContainerID: 0, Size: 1}}

	// The first path uses the fast path (reading /proc/<pid>/*id_map),
	// the second one uses the slow path (joining the user namespace).
	for _, path := range []string{
		"/proc/" + pid + "/ns/user",
		"/proc/" + pid + "/task/" + pid + "/ns/user",
	} {
		uidMap, gidMap, err := GetUserNamespaceMappings(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		want[0].HostID = int64(os.Geteuid())
		if !IsSameMapping(uidMap, want) {
			t.Errorf("%s: want uid map %+v, got %+v", path, want, uidMap)
		}
		want[0].HostID = int64(os.Getegid())
		if !IsSameMapping(gidMap, want) {
			t.Errorf("%s: want gid map %+v, got %+v", path, want, gidMap)
		}
	}
}

func TestSpawnUserNamespaceCatError(t *testing.T) {
	pid := strconv.Itoa(startUserns(t))
	nsPath := "/proc/" + pid + "/ns/user"

	_, err := spawnUserNamespaceCat(nsPath, "/no/such/file")
	if !errors.Is(err, unix.ENOENT) || !strings.Contains(err.Error(), "open /no/such/file") {
		t.Errorf("want open ENOENT error, got %v", err)
	}

	_, err = spawnUserNamespaceCat("/proc/self/ns/net", "/proc/self/uid_map")
	if !errors.Is(err, unix.EINVAL) || !strings.Contains(err.Error(), "setns /proc/self/ns/net") {
		t.Errorf("want setns EINVAL error, got %v", err)
	}
}
