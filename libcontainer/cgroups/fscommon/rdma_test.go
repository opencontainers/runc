package fscommon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

/* Roadmap for future */
// (Low-priority)  TODO: Check if it is possible to virtually mimic an actual RDMA device.
// TODO: Think of more edge-cases to add.

// TestRdmaSet performs an E2E test of RdmaSet(), parseRdmaKV() using dummy device and a dummy cgroup file-system.
// Note: Following test does not guarantees that your host supports RDMA since this mocks underlying infrastructure.
func TestRdmaSet(t *testing.T) {
	testCgroupPath := filepath.Join(t.TempDir(), "rdma")

	// Ensure the full mock cgroup path exists.
	err := os.Mkdir(testCgroupPath, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	rdmaDevice := "mlx5_1"
	maxHandles := uint32(100)
	maxObjects := uint32(300)

	rdmaStubResource := &configs.Resources{
		Rdma: map[string]configs.LinuxRdma{
			rdmaDevice: {
				HcaHandles: &maxHandles,
				HcaObjects: &maxObjects,
			},
		},
	}

	if err := RdmaSet(testCgroupPath, rdmaStubResource); err != nil {
		t.Fatal(err)
	}

	// The default rdma.max must be written.
	rdmaEntries, err := readRdmaEntries(testCgroupPath, "rdma.max")
	if err != nil {
		t.Fatal(err)
	}
	if len(rdmaEntries) != 1 {
		t.Fatal("rdma_test: Got the wrong values while parsing entries from rdma.max")
	}
	if rdmaEntries[0].HcaHandles != maxHandles {
		t.Fatalf("rdma_test: Got the wrong value for hca_handles")
	}
	if rdmaEntries[0].HcaObjects != maxObjects {
		t.Fatalf("rdma_test: Got the wrong value for hca_Objects")
	}
}
