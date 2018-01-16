// +build linux

package system

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/user"
)

func TestUIDMapInUserNS(t *testing.T) {
	cases := []struct {
		s        string
		expected bool
	}{
		{
			s:        "         0          0 4294967295\n",
			expected: false,
		},
		{
			s:        "         0          0          1\n",
			expected: true,
		},
		{
			s:        "         0       1001          1\n         1     231072      65536\n",
			expected: true,
		},
		{
			// file exist but empty (the initial state when userns is created. see man 7 user_namespaces)
			s:        "",
			expected: true,
		},
	}
	for _, c := range cases {
		uidmap, err := user.ParseIDMap(strings.NewReader(c.s))
		if err != nil {
			t.Fatal(err)
		}
		actual := UIDMapInUserNS(uidmap)
		if c.expected != actual {
			t.Fatalf("expected %v, got %v for %q", c.expected, actual, c.s)
		}
	}
}
