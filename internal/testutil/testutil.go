package testutil

import (
	"os/exec"
	"strconv"
	"sync"
	"testing"
)

var (
	centosVer     string
	centosVerOnce sync.Once
)

func centosVersion() string {
	centosVerOnce.Do(func() {
		ver, _ := exec.Command("rpm", "-q", "--qf", "%{version}", "centos-release").CombinedOutput()
		centosVer = string(ver)
	})
	return centosVer
}

func SkipOnCentOS(t *testing.T, reason string, versions ...int) {
	t.Helper()
	for _, v := range versions {
		if vstr := strconv.Itoa(v); centosVersion() == vstr {
			t.Skip(reason + " on CentOS " + vstr)
		}
	}
}
