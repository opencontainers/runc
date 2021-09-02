package cgroups

import (
	"testing"
)

func BenchmarkGetAllPids(b *testing.B) {
	total := 0
	for i := 0; i < b.N; i++ {
		i, err := GetAllPids("/sys/fs/cgroup")
		if err != nil {
			b.Fatal(err)
		}
		total += len(i)
	}
	b.Logf("iter: %d, total: %d", b.N, total)
}
