// +build linux

package fs

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func TestIntelRdtSetL3CacheSchema(t *testing.T) {
	if !IsIntelRdtEnabled() {
		return
	}

	helper := NewCgroupTestUtil("intel_rdt", t)
	defer helper.cleanup()

	const (
		l3CacheSchemaBefore = "L3:0=f;1=f0"
		l3CacheSchemeAfter  = "L3:0=f0;1=f"
	)

	helper.writeFileContents(map[string]string{
		"schemata": l3CacheSchemaBefore + "\n",
	})

	helper.CgroupData.config.Resources.IntelRdtL3CacheSchema = l3CacheSchemeAfter
	intelrdt := &IntelRdtGroup{}
	if err := intelrdt.Set(helper.CgroupPath, helper.CgroupData.config); err != nil {
		t.Fatal(err)
	}

	value, err := getCgroupParamString(helper.CgroupPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}

	if value != l3CacheSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

func TestIntelRdtStats(t *testing.T) {
	if !IsIntelRdtEnabled() {
		return
	}

	helper := NewCgroupTestUtil("intel_rdt", t)
	defer helper.cleanup()

	const (
		l3CacheSchemaContent = "L3:0=ffff0;1=fff00"
	)

	helper.writeFileContents(map[string]string{
		"schemata": l3CacheSchemaContent + "\n",
	})

	intelrdt := &IntelRdtGroup{}
	stats := *cgroups.NewStats()
	if err := intelrdt.GetStats(helper.CgroupPath, &stats); err != nil {
		t.Fatal(err)
	}

	if stats.IntelRdtStats.IntelRdtGroupStats.L3CacheSchema != l3CacheSchemaContent {
		t.Fatalf("Expected '%q', got '%q' for file 'schemata'",
			l3CacheSchemaContent, stats.IntelRdtStats.IntelRdtGroupStats.L3CacheSchema)
	}
}
