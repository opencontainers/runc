package intelrdt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestIntelRdtSetL3CacheSchema(t *testing.T) {
	helper := NewIntelRdtTestUtil(t)

	const (
		l3CacheSchemaBefore = "L3:0=f;1=f0"
		l3CacheSchemeAfter  = "L3:0=f0;1=f"
	)

	helper.writeFileContents(map[string]string{
		"schemata": l3CacheSchemaBefore + "\n",
	})

	helper.config.IntelRdt.L3CacheSchema = l3CacheSchemeAfter
	intelrdt := newManager(helper.config, "", helper.IntelRdtPath)
	if err := intelrdt.Set(helper.config); err != nil {
		t.Fatal(err)
	}

	tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}
	values := strings.Split(tmpStrings, "\n")
	value := values[0]

	if value != l3CacheSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

func TestIntelRdtSetMemBwSchema(t *testing.T) {
	helper := NewIntelRdtTestUtil(t)

	const (
		memBwSchemaBefore = "MB:0=20;1=70"
		memBwSchemeAfter  = "MB:0=70;1=20"
	)

	helper.writeFileContents(map[string]string{
		"schemata": memBwSchemaBefore + "\n",
	})

	helper.config.IntelRdt.MemBwSchema = memBwSchemeAfter
	intelrdt := newManager(helper.config, "", helper.IntelRdtPath)
	if err := intelrdt.Set(helper.config); err != nil {
		t.Fatal(err)
	}

	tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}
	values := strings.Split(tmpStrings, "\n")
	value := values[0]

	if value != memBwSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

func TestIntelRdtSetMemBwScSchema(t *testing.T) {
	helper := NewIntelRdtTestUtil(t)

	const (
		memBwScSchemaBefore = "MB:0=5000;1=7000"
		memBwScSchemeAfter  = "MB:0=9000;1=4000"
	)

	helper.writeFileContents(map[string]string{
		"schemata": memBwScSchemaBefore + "\n",
	})

	helper.config.IntelRdt.MemBwSchema = memBwScSchemeAfter
	intelrdt := newManager(helper.config, "", helper.IntelRdtPath)
	if err := intelrdt.Set(helper.config); err != nil {
		t.Fatal(err)
	}

	tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
	if err != nil {
		t.Fatalf("Failed to parse file 'schemata' - %s", err)
	}
	values := strings.Split(tmpStrings, "\n")
	value := values[0]

	if value != memBwScSchemeAfter {
		t.Fatal("Got the wrong value, set 'schemata' failed.")
	}
}

func TestApply(t *testing.T) {
	const closID = "test-clos"
	// TC-1: failure because non-pre-existing CLOS
	{
		helper := NewIntelRdtTestUtil(t)
		helper.config.IntelRdt = &configs.IntelRdt{
			ClosID: closID,
		}

		intelrdt := newManager(helper.config, "", "")
		if err := intelrdt.Apply(1234); err == nil {
			t.Fatal("unexpected success when applying pid")
		}
		closPath := filepath.Join(intelRdtRoot, closID)
		if _, err := os.Stat(closPath); err == nil {
			t.Fatal("closid dir should not exist")
		}
	}
	// TC-2: CLOS dir should be created if some schema has been specified
	{
		helper := NewIntelRdtTestUtil(t)
		helper.config.IntelRdt = &configs.IntelRdt{
			ClosID:        closID,
			L3CacheSchema: "L3:0=f",
		}

		intelrdt := newManager(helper.config, "", "")
		if err := intelrdt.Apply(1235); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		closPath := filepath.Join(intelRdtRoot, closID)
		pids, err := getIntelRdtParamString(closPath, "tasks")
		if err != nil {
			t.Fatalf("failed to read tasks file: %v", err)
		}
		if pids != "1235" {
			t.Fatalf("unexpected tasks file, expected '1235', got %q", pids)
		}
	}
	// TC-3: clos and monitoring group should be created if EnableMonitoring is true
	{
		helper := NewIntelRdtTestUtil(t)
		helper.config.IntelRdt = &configs.IntelRdt{
			EnableMonitoring: true,
		}
		id := "aaaa-bbbb"

		intelrdt := newManager(helper.config, id, "")
		// We need to pre-create the CLOS/mon_groups directory
		closPath := filepath.Join(intelRdtRoot, id)
		if err := os.MkdirAll(filepath.Join(closPath, "mon_groups"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := intelrdt.Apply(1236); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}

		pids, err := getIntelRdtParamString(closPath, "tasks")
		if err != nil {
			t.Fatalf("failed to read tasks file: %v", err)
		}
		if pids != "1236" {
			t.Fatalf("unexpected tasks file, expected '1236', got %q", pids)
		}
	}
}

func TestDestroy(t *testing.T) {
	const closID = "test-clos"

	// TC-1: per-container CLOS dir should be removed
	{
		helper := NewIntelRdtTestUtil(t)
		id := "abcd-efgh"

		intelrdt := newManager(helper.config, id, "")
		if err := intelrdt.Apply(1234); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}
		closPath := filepath.Join(intelRdtRoot, id)
		if _, err := os.Stat(closPath); err != nil {
			t.Fatal("CLOS dir should exist")
		}
		// Need to delete the tasks file so that the dir is empty
		os.Remove(filepath.Join(closPath, "tasks"))
		if err := intelrdt.Destroy(); err != nil {
			t.Fatalf("Destroy() failed: %v", err)
		}
		if _, err := os.Stat(closPath); err == nil {
			t.Fatal("CLOS dir should not exist")
		}
	}
	// TC-2: pre-existing CLOS should not be removed
	{
		helper := NewIntelRdtTestUtil(t)
		helper.config.IntelRdt = &configs.IntelRdt{
			ClosID: closID,
		}

		closPath := filepath.Join(intelRdtRoot, closID)
		if err := os.MkdirAll(closPath, 0o755); err != nil {
			t.Fatal(err)
		}

		intelrdt := newManager(helper.config, "", "")
		if err := intelrdt.Apply(1234); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}
		if _, err := os.Stat(closPath); err != nil {
			t.Fatal("CLOS dir should exist")
		}
		if err := intelrdt.Destroy(); err != nil {
			t.Fatalf("Destroy() failed: %v", err)
		}
		if _, err := os.Stat(closPath); err != nil {
			t.Fatal("CLOS dir should exist")
		}
	}
	// TC-3: per-container MON dir in pre-existing CLOS should be removed
	{
		helper := NewIntelRdtTestUtil(t)
		helper.config.IntelRdt = &configs.IntelRdt{
			ClosID:           closID,
			EnableMonitoring: true,
		}
		id := "abcd-efgh"

		closPath := filepath.Join(intelRdtRoot, closID)
		if err := os.MkdirAll(filepath.Join(closPath, "mon_groups"), 0o755); err != nil {
			t.Fatal(err)
		}

		intelrdt := newManager(helper.config, id, "")
		if err := intelrdt.Apply(1234); err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}
		monPath := filepath.Join(closPath, "mon_groups", id)
		if _, err := os.Stat(monPath); err != nil {
			t.Fatal("MON dir should exist")
		}
		// Need to delete the tasks file so that the dir is empty
		os.Remove(filepath.Join(monPath, "tasks"))
		if err := intelrdt.Destroy(); err != nil {
			t.Fatalf("Destroy() failed: %v", err)
		}
		if _, err := os.Stat(closPath); err != nil {
			t.Fatalf("CLOS dir should exist: %f", err)
		}
		if _, err := os.Stat(monPath); err == nil {
			t.Fatal("MON dir should not exist")
		}
	}
}
