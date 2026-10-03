package sys

import "testing"

func TestSysctlKeyPath(t *testing.T) {
	for _, tc := range []struct {
		key  string
		path string
	}{
		// Common case: dot-form key, no literal dots inside a component.
		{"net.ipv4.ip_forward", "net/ipv4/ip_forward"},
		{"kernel.shmmax", "kernel/shmmax"},
		// Dot-form key with a literal dot in an interface name, escaped
		// with '/' per sysctl(8).
		{"net.ipv4.conf.eth0/100.rp_filter", "net/ipv4/conf/eth0.100/rp_filter"},
		// Slash-form spelling of the same key.
		{"net/ipv4/conf/eth0.100/rp_filter", "net/ipv4/conf/eth0.100/rp_filter"},
	} {
		if got := sysctlKeyPath(tc.key); got != tc.path {
			t.Errorf("sysctlKeyPath(%q) = %q, want %q", tc.key, got, tc.path)
		}
	}
}
