package intelrdt

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	const pid = 1234
	tests := []struct {
		name            string
		config          configs.IntelRdt
		precreateClos   bool
		isError         bool
		postApplyAssert func(*Manager)
	}{
		{
			name: "failure because non-pre-existing CLOS",
			config: configs.IntelRdt{
				ClosID: "non-existing-clos",
			},
			isError: true,
			postApplyAssert: func(m *Manager) {
				if _, err := os.Stat(m.path); err == nil {
					t.Fatal("closid dir should not exist")
				}
			},
		},
		{
			name: "CLOS dir should be created if some schema has been specified",
			config: configs.IntelRdt{
				ClosID:        "clos-to-be-created",
				L3CacheSchema: "L3:0=f",
			},
			postApplyAssert: func(m *Manager) {
				pids, err := getIntelRdtParamString(m.path, "tasks")
				if err != nil {
					t.Fatalf("failed to read tasks file: %v", err)
				}
				if pids != strconv.Itoa(pid) {
					t.Fatalf("unexpected tasks file, expected '%d', got %q", pid, pids)
				}
			},
		},
		{
			name: "clos and monitoring group should be created if EnableMonitoring is true",
			config: configs.IntelRdt{
				EnableMonitoring: true,
			},
			precreateClos: true,
			postApplyAssert: func(m *Manager) {
				pids, err := getIntelRdtParamString(m.path, "tasks")
				if err != nil {
					t.Fatalf("failed to read tasks file: %v", err)
				}
				if pids != strconv.Itoa(pid) {
					t.Fatalf("unexpected tasks file, expected '%d', got %q", pid, pids)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NewIntelRdtTestUtil(t)
			id := "abcd-1234"
			closPath := filepath.Join(intelRdtRoot, id)
			if tt.config.ClosID != "" {
				closPath = filepath.Join(intelRdtRoot, tt.config.ClosID)
			}

			if tt.precreateClos {
				if err := os.MkdirAll(filepath.Join(closPath, "mon_groups"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			m := newManager(&configs.Config{IntelRdt: &tt.config}, id, closPath)
			err := m.Apply(pid)
			if tt.isError && err == nil {
				t.Fatal("expected error, got nil")
			} else if !tt.isError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.postApplyAssert(m)
		})
	}
}

func TestDestroy(t *testing.T) {
	tests := []struct {
		name     string
		config   configs.IntelRdt
		testFunc func(*Manager)
	}{
		{
			name: "per-container CLOS dir should be removed",
			testFunc: func(m *Manager) {
				closPath := m.path
				if _, err := os.Stat(closPath); err != nil {
					t.Fatal("CLOS dir should exist")
				}
				// Need to delete the tasks file so that the dir is empty
				if err := os.Remove(filepath.Join(closPath, "tasks")); err != nil {
					t.Fatalf("failed to remove tasks file: %v", err)
				}
				if err := m.Destroy(); err != nil {
					t.Fatalf("Destroy() failed: %v", err)
				}
				if _, err := os.Stat(closPath); err == nil {
					t.Fatal("CLOS dir should not exist")
				}
			},
		},
		{
			name: "pre-existing CLOS should not be removed",
			config: configs.IntelRdt{
				ClosID: "pre-existing-clos",
			},
			testFunc: func(m *Manager) {
				closPath := m.path

				if _, err := os.Stat(closPath); err != nil {
					t.Fatal("CLOS dir should exist")
				}
				if err := m.Destroy(); err != nil {
					t.Fatalf("Destroy() failed: %v", err)
				}
				if _, err := os.Stat(closPath); err != nil {
					t.Fatal("CLOS dir should exist")
				}
			},
		},
		{
			name: "per-container MON dir in pre-existing CLOS should be removed",
			config: configs.IntelRdt{
				ClosID:           "pre-existing-clos",
				EnableMonitoring: true,
			},
			testFunc: func(m *Manager) {
				closPath := m.path

				monPath := filepath.Join(closPath, "mon_groups", m.id)
				if _, err := os.Stat(monPath); err != nil {
					t.Fatal("MON dir should exist")
				}
				// Need to delete the tasks file so that the dir is empty
				os.Remove(filepath.Join(monPath, "tasks"))
				if err := m.Destroy(); err != nil {
					t.Fatalf("Destroy() failed: %v", err)
				}
				if _, err := os.Stat(closPath); err != nil {
					t.Fatalf("CLOS dir should exist: %f", err)
				}
				if _, err := os.Stat(monPath); err == nil {
					t.Fatal("MON dir should not exist")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NewIntelRdtTestUtil(t)

			id := "abcd-1234"
			closPath := filepath.Join(intelRdtRoot, id)
			if tt.config.ClosID != "" {
				closPath = filepath.Join(intelRdtRoot, tt.config.ClosID)
				// Pre-create the CLOS directory
				if err := os.MkdirAll(filepath.Join(closPath, "mon_groups"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			m := newManager(&configs.Config{IntelRdt: &tt.config}, id, closPath)
			if err := m.Apply(1234); err != nil {
				t.Fatalf("Apply() failed: %v", err)
			}
			tt.testFunc(m)
		})
	}
}
