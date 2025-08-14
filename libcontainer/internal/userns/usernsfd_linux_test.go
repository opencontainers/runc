package userns

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func BenchmarkSpawnProc(b *testing.B) {
	if os.Geteuid() != 0 {
		b.Skip("spawning user namespaced processes requires root")
	}

	// We can reuse the mapping as we call spawnProc() directly.
	mapping := Mapping{
		UIDMappings: []configs.IDMap{
			{ContainerID: 0, HostID: 1337, Size: 142},
			{ContainerID: 150, HostID: 0, Size: 1},
			{ContainerID: 442, HostID: 1111, Size: 12},
			{ContainerID: 1000, HostID: 9999, Size: 92},
			{ContainerID: 9999, HostID: 1000000, Size: 4},
			// Newer kernels support more than 5 entries, but stick to 5 here.
		},
		GIDMappings: []configs.IDMap{
			{ContainerID: 1, HostID: 2337, Size: 142},
			{ContainerID: 145, HostID: 6, Size: 1},
			{ContainerID: 200, HostID: 1000, Size: 12},
			{ContainerID: 1000, HostID: 9888, Size: 92},
			{ContainerID: 8999, HostID: 1000000, Size: 4},
			// Newer kernels support more than 5 entries, but stick to 5 here.
		},
	}

	procs := make([]*os.Process, 0, b.N)
	b.Cleanup(func() {
		for _, proc := range procs {
			if proc != nil {
				_ = proc.Kill()
				_, _ = proc.Wait()
			}
		}
	})

	for b.Loop() {
		proc, err := spawnProc(mapping)
		if err != nil {
			b.Error(err)
		}
		procs = append(procs, proc)
	}
}
