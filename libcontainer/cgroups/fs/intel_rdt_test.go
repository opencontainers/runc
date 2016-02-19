// +build linux

package fs

import (
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func TestIntelRdtSetL3Cbm(t *testing.T) {
	helper := NewCgroupTestUtil("intel_rdt", t)
	defer helper.cleanup()

	const (
		l3CbmBefore = 0xf
		l3CbmAfter  = 0xf0
	)

	helper.writeFileContents(map[string]string{
		"intel_rdt.l3_cbm": strconv.FormatUint(l3CbmBefore, 16),
	})

	helper.CgroupData.config.Resources.IntelRdtL3Cbm = l3CbmAfter
	intelrdt := &IntelRdtGroup{}
	if err := intelrdt.Set(helper.CgroupPath, helper.CgroupData.config); err != nil {
		t.Fatal(err)
	}

	value, err := getCgroupParamUintHex(helper.CgroupPath, "intel_rdt.l3_cbm")
	if err != nil {
		t.Fatalf("Failed to parse intel_rdt.l3_cbm - %s", err)
	}

	if value != l3CbmAfter {
		t.Fatal("Got the wrong value, set intel_rdt.l3_cbm failed.")
	}
}

func TestIntelRdtStats(t *testing.T) {
	helper := NewCgroupTestUtil("intel_rdt", t)
	defer helper.cleanup()

	const (
		l3CbmContents = 0x1f00
	)

	helper.writeFileContents(map[string]string{
		"intel_rdt.l3_cbm": strconv.FormatUint(l3CbmContents, 16),
	})

	intelrdt := &IntelRdtGroup{}
	stats := *cgroups.NewStats()
	if err := intelrdt.GetStats(helper.CgroupPath, &stats); err != nil {
		t.Fatal(err)
	}

	if stats.IntelRdtStats.L3Cbm != l3CbmContents {
		t.Fatalf("Expected '0x%x', got '0x%x' for intel_rdt.l3_cbm", l3CbmContents, stats.IntelRdtStats.L3Cbm)
	}
}
