package intelrdt

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestIntelRdtSet(t *testing.T) {
	tcs := []struct {
		name          string
		config        *configs.IntelRdt
		schemataAfter []string
	}{
		{
			name: "L3",
			config: &configs.IntelRdt{
				L3CacheSchema: "L3:0=f0;1=f",
			},
			schemataAfter: []string{"L3:0=f0;1=f"},
		},
		{
			name: "MemBw",
			config: &configs.IntelRdt{
				MemBwSchema: "MB:0=70;1=20",
			},
			schemataAfter: []string{"MB:0=70;1=20"},
		},
		{
			name: "MemBwSc",
			config: &configs.IntelRdt{
				MemBwSchema: "MB:0=9000;1=4000",
			},
			schemataAfter: []string{"MB:0=9000;1=4000"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			helper := NewIntelRdtTestUtil(t)
			helper.config.IntelRdt = tc.config

			helper.writeFileContents(map[string]string{
				/* Common initial value for all test cases */
				"schemata": "MB:0=100\nL3:0=ffff\nL2:0=ffffffff\n",
			})

			intelrdt := newManager(helper.config, "", helper.IntelRdtPath)
			if err := intelrdt.Set(helper.config); err != nil {
				t.Fatal(err)
			}

			tmpStrings, err := getIntelRdtParamString(helper.IntelRdtPath, "schemata")
			if err != nil {
				t.Fatalf("Failed to parse file 'schemata' - %s", err)
			}
			values := strings.Split(tmpStrings, "\n")

			if slices.Compare(values, tc.schemataAfter) != 0 {
				t.Fatalf("Got the wrong value, expected %v, got %v", tc.schemataAfter, values)
			}
		})
	}
}

func TestApply(t *testing.T) {
	helper := NewIntelRdtTestUtil(t)

	const closID = "test-clos"
	closPath := filepath.Join(helper.IntelRdtPath, closID)

	helper.config.IntelRdt.ClosID = closID
	intelrdt := newManager(helper.config, "container-1", closPath)
	if err := intelrdt.Apply(1234); err == nil {
		t.Fatal("unexpected success when applying pid")
	}
	if _, err := os.Stat(closPath); err == nil {
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
