/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package fs2

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func TestParseCgroupFromReader(t *testing.T) {
	cases := map[string]string{
		"0::/user.slice/user-1001.slice/session-1.scope\n":                                  "/user.slice/user-1001.slice/session-1.scope",
		"2:cpuset:/foo\n1:name=systemd:/\n":                                                 "",
		"2:cpuset:/foo\n1:name=systemd:/\n0::/user.slice/user-1001.slice/session-1.scope\n": "/user.slice/user-1001.slice/session-1.scope",
	}
	for s, expected := range cases {
		g, err := parseCgroupFromReader(strings.NewReader(s))
		if expected != "" {
			if g != expected {
				t.Errorf("expected %q, got %q", expected, g)
			}
			if err != nil {
				t.Error(err)
			}
		} else {
			if err == nil {
				t.Error("error is expected")
			}
		}
	}
}

func TestDefaultDirPath(t *testing.T) {
	if !cgroups.IsCgroup2UnifiedMode() {
		t.Skip("need cgroupv2")
	}
	// same code as in defaultDirPath()
	ownCgroup, err := parseCgroupFile("/proc/self/cgroup")
	if err != nil {
		// Not a test failure, but rather some weird
		// environment so we can't run this test.
		t.Skipf("can't get own cgroup: %v", err)
	}
	ownCgroup = filepath.Dir(ownCgroup)

	cases := []struct {
		cgPath   string
		cgParent string
		cgName   string
		expected string
	}{
		{
			cgPath:   "/foo/bar",
			expected: "/sys/fs/cgroup/foo/bar",
		},
		{
			cgPath:   "foo/bar",
			expected: filepath.Join(UnifiedMountpoint, ownCgroup, "foo/bar"),
		},
	}
	for _, c := range cases {
		got, err := _defaultDirPath(UnifiedMountpoint, c.cgPath, c.cgParent, c.cgName)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.expected {
			t.Fatalf("expected %q, got %q", c.expected, got)
		}
	}
}
