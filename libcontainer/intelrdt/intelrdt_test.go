package intelrdt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	helper := NewIntelRdtTestUtil(t)

	const closID = "test-clos"

	helper.config.IntelRdt.ClosID = closID
	intelrdt := newManager(helper.config, "", helper.IntelRdtPath)
	if err := intelrdt.Apply(1234); err == nil {
		t.Fatal("unexpected success when applying pid")
	}
	if _, err := os.Stat(filepath.Join(helper.IntelRdtPath, closID)); err == nil {
		t.Fatal("closid dir should not exist")
	}

	// Dir should be created if some schema has been specified
	intelrdt.config.IntelRdt.L3CacheSchema = "L3:0=f"
	if err := intelrdt.Apply(1235); err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	pids, err := getIntelRdtParamString(intelrdt.GetPath(), "tasks")
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	if pids != "1235" {
		t.Fatalf("unexpected tasks file, expected '1235', got %q", pids)
	}
}

func TestIntelRdtManagerSetSchemataInMonGroup(t *testing.T) {
	helper := NewIntelRdtTestUtil(t)

	intelrdt := Manager{
		mu:              sync.Mutex{},
		config:          helper.config,
		id:              "",
		path:            helper.IntelRdtPath,
		monitoringGroup: true,
	}

	test := []struct {
		l3Schema    string
		memBwSchema string
	}{
		{
			"L3:0=f0;1=f",
			"",
		},
		{
			"L3:0=f0;1=f",
			"MB:0=20;1=70",
		},
		{
			"",
			"MB:0=20;1=70",
		},
	}

	expectedError := errors.New("couldn't set IntelRdt l3CacheSchema or memBwSchema for the monitoring group")

	for _, tc := range test {
		err := intelrdt.Set(&configs.Config{IntelRdt: &configs.IntelRdt{
			L3CacheSchema: tc.l3Schema,
			MemBwSchema:   tc.memBwSchema,
		}})

		if err == nil {
			t.Fatalf("Expected error: %v, got nil.", expectedError)
		}

		if err.Error() != expectedError.Error() {
			t.Fatalf("Expected error: %v but got: %v.", expectedError, err)
		}
	}
}
