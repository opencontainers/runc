package fscommon

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

/* Roadmap for future */
// (Low-priority)  TODO: Check if it is possible to virtually mimic an actual RDMA device.
// TODO: Think of more edge-cases to add.


// TestRdmaSet performs an E2E test of RdmaSet(), parseRdmaKV() using dummy device and a dummy cgroup file-system.
// Note: Following test does not gurantees that your host supports RDMA since this mocks underlying infrastructure.
func TestRdmaSet(t *testing.T) {
	helper := NewCgroupTestUtil("rdma", t)
	defer helper.cleanup()

	helper.writeFileContents(map[string]string{
		"rdma.max": "",
	})

	rdmaDevice := "mlx5_1"
	maxHandles := uint32(100)
	maxObjects := uint32(300)
	helper.CgroupData.config.Resources.Rdma = map[string]configs.LinuxRdma{rdmaDevice: {&maxHandles, &maxObjects}}

	if err := RdmaSet(helper.CgroupPath, helper.CgroupData.config.Resources); err != nil {
		t.Fatal(err)
	}

	// The default rdma.max must be written.
	value, err := GetCgroupParamString(helper.CgroupPath, "rdma.max")
	if err != nil {
		t.Fatalf("rdma_test: Failed to parse rdma.max: %s", err)
	}
	z := strings.SplitN(value, " ", 3)
	if len(z) != 3 {
		t.Errorf("rdma_test: Invalid entry found in rdma.max")
	}

	var rdmaEntry cgroups.RdmaEntry

	if z[0] != rdmaDevice {
		t.Fatalf("rdma_test: Got the wrong value for device name")
	}

	err = parseRdmaKV(z[1], &rdmaEntry)
	if err != nil {
		t.Fatalf("Failed while parsing RDMA entry: %s", err)
	}
	err = parseRdmaKV(z[2], &rdmaEntry)
	if err != nil {
		t.Fatalf("Failed while parsing RDMA entry: %s", err)
	}
	if rdmaEntry.HcaHandles != maxHandles {
		t.Fatalf("rdma_test: Got the wrong value for hca_handles")
	}
	if rdmaEntry.HcaObjects != maxObjects {
		t.Fatalf("rdma_test: Got the wrong value for hca_Objects")
	}
}
