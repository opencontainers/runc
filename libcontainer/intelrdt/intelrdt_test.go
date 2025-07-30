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
		{
			name: "L3 and MemBw",
			config: &configs.IntelRdt{
				L3CacheSchema: "L3:0=f0;1=f",
				MemBwSchema:   "MB:0=9000;1=4000",
			},
			schemataAfter: []string{
				"L3:0=f0;1=f",
				"MB:0=9000;1=4000",
			},
		},
		{
			name: "Schemata",
			config: &configs.IntelRdt{
				Schemata: []string{
					"L3CODE:0=ff;1=ff",
					"L3DATA:0=f;1=f0",
				},
			},
			schemataAfter: []string{
				"L3CODE:0=ff;1=ff",
				"L3DATA:0=f;1=f0",
			},
		},
		{
			name: "Schemata and L3",
			config: &configs.IntelRdt{
				L3CacheSchema: "L3:0=f0;1=f",
				Schemata:      []string{"L2:0=ff00;1=ff"},
			},
			schemataAfter: []string{
				"L3:0=f0;1=f",
				"L2:0=ff00;1=ff",
			},
		},
		{
			name: "Schemata and MemBw",
			config: &configs.IntelRdt{
				MemBwSchema: "MB:0=2000;1=4000",
				Schemata:    []string{"L3:0=ff;1=ff"},
			},
			schemataAfter: []string{
				"MB:0=2000;1=4000",
				"L3:0=ff;1=ff",
			},
		},
		{
			name: "Schemata, L3 and MemBw",
			config: &configs.IntelRdt{
				L3CacheSchema: "L3:0=80;1=7f",
				MemBwSchema:   "MB:0=2000;1=4000",
				Schemata: []string{
					"L2:0=ff00;1=ff",
					"L3:0=c0;1=3f",
				},
			},
			schemataAfter: []string{
				"L3:0=80;1=7f",
				"MB:0=2000;1=4000",
				"L2:0=ff00;1=ff",
				"L3:0=c0;1=3f",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			helper := NewIntelRdtTestUtil(t)
			helper.config.IntelRdt = tc.config

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
