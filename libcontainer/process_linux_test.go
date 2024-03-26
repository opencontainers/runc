package libcontainer

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestIsolatedCPUAffinityTransition(t *testing.T) {
	const isolatedCPUAffinityTransitionAnnotation = "org.opencontainers.runc.exec.isolated-cpu-affinity-transition"

	noAffinity := -1
	temporaryTransition := "temporary"
	definitiveTransition := "definitive"

	tests := []struct {
		name                         string
		testFS                       fs.FS
		cpuset                       string
		expectedErr                  bool
		expectedAffinityCore         int
		expectedDefinitiveTransition bool
		annotations                  map[string]string
	}{
		{
			name:   "no affinity",
			cpuset: "0-15",
			testFS: fstest.MapFS{
				"sys/devices/system/cpu/nohz_full": &fstest.MapFile{Data: []byte("0-4\n")},
			},
			expectedAffinityCore:         noAffinity,
			expectedDefinitiveTransition: false,
		},
		{
			name:   "affinity match with temporary transition",
			cpuset: "3-4",
			testFS: fstest.MapFS{
				"sys/devices/system/cpu/nohz_full": &fstest.MapFile{Data: []byte("0-4\n")},
			},
			expectedAffinityCore:         3,
			expectedDefinitiveTransition: false,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: temporaryTransition,
			},
		},
		{
			name:   "affinity match with temporary transition and nohz_full boot param",
			cpuset: "3-4",
			testFS: fstest.MapFS{
				"proc/cmdline": &fstest.MapFile{Data: []byte("nohz_full=0-4\n")},
			},
			expectedAffinityCore:         3,
			expectedDefinitiveTransition: false,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: temporaryTransition,
			},
		},
		{
			name:   "affinity match with definitive transition",
			cpuset: "3-4",
			testFS: fstest.MapFS{
				"sys/devices/system/cpu/nohz_full": &fstest.MapFile{Data: []byte("0-4\n")},
			},
			expectedAffinityCore:         3,
			expectedDefinitiveTransition: true,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: definitiveTransition,
			},
		},
		{
			name:   "affinity match with definitive transition and nohz_full boot param",
			cpuset: "3-4",
			testFS: fstest.MapFS{
				"proc/cmdline": &fstest.MapFile{Data: []byte("nohz_full=0-4\n")},
			},
			expectedAffinityCore:         3,
			expectedDefinitiveTransition: true,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: definitiveTransition,
			},
		},
		{
			name:   "affinity error with bad isolated set",
			cpuset: "0-15",
			testFS: fstest.MapFS{
				"sys/devices/system/cpu/nohz_full": &fstest.MapFile{Data: []byte("bad_isolated_set\n")},
			},
			expectedErr:          true,
			expectedAffinityCore: noAffinity,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: temporaryTransition,
			},
		},
		{
			name:   "affinity error with bad isolated set for nohz_full boot param",
			cpuset: "0-15",
			testFS: fstest.MapFS{
				"proc/cmdline": &fstest.MapFile{Data: []byte("nohz_full=bad_isolated_set\n")},
			},
			expectedErr:          true,
			expectedAffinityCore: noAffinity,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: temporaryTransition,
			},
		},
		{
			name:   "no affinity with null isolated set value",
			cpuset: "0-15",
			testFS: fstest.MapFS{
				"sys/devices/system/cpu/nohz_full": &fstest.MapFile{Data: []byte("(null)\n")},
			},
			expectedAffinityCore:         noAffinity,
			expectedDefinitiveTransition: false,
			annotations: map[string]string{
				isolatedCPUAffinityTransitionAnnotation: temporaryTransition,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			affinityCore, definitive, err := isolatedCPUAffinityTransition(tt.testFS, tt.cpuset, tt.annotations)
			if err != nil && !tt.expectedErr {
				t.Fatalf("unexpected error: %s", err)
			} else if err == nil && tt.expectedErr {
				t.Fatalf("unexpected success")
			} else if tt.expectedDefinitiveTransition != definitive {
				t.Fatalf("expected reset affinity %t: got %t instead", tt.expectedDefinitiveTransition, definitive)
			} else if tt.expectedAffinityCore != affinityCore {
				t.Fatalf("expected affinity core %d: got %d instead", tt.expectedAffinityCore, affinityCore)
			}
		})
	}
}

func TestGetEligibleCPU(t *testing.T) {
	tests := []struct {
		name                 string
		cpuset               string
		isolset              string
		expectedErr          bool
		expectedAffinityCore int
		expectedEligible     bool
	}{
		{
			name:             "no cpuset",
			isolset:          "2-15,18-31,34-47",
			expectedEligible: false,
		},
		{
			name:             "no isolated set",
			cpuset:           "0-15",
			expectedEligible: false,
		},
		{
			name:        "bad cpuset format",
			cpuset:      "core0 to core15",
			isolset:     "2-15,18-31,34-47",
			expectedErr: true,
		},
		{
			name:        "bad isolated set format",
			cpuset:      "0-15",
			isolset:     "core0 to core15",
			expectedErr: true,
		},
		{
			name:             "no eligible core",
			cpuset:           "0-1,16-17,32-33",
			isolset:          "2-15,18-31,34-47",
			expectedEligible: false,
		},
		{
			name:             "no eligible core inverted",
			cpuset:           "2-15,18-31,34-47",
			isolset:          "0-1,16-17,32-33",
			expectedEligible: false,
		},
		{
			name:                 "eligible core mixed",
			cpuset:               "8-31",
			isolset:              "2-15,18-31,34-47",
			expectedEligible:     true,
			expectedAffinityCore: 16,
		},
		{
			name:                 "eligible core #4",
			cpuset:               "4-7",
			isolset:              "2-15,18-31,34-47",
			expectedEligible:     true,
			expectedAffinityCore: 4,
		},
		{
			name:                 "eligible core #40",
			cpuset:               "40-47",
			isolset:              "2-15,18-31,34-47",
			expectedEligible:     true,
			expectedAffinityCore: 40,
		},
		{
			name:                 "eligible core #24",
			cpuset:               "24-31",
			isolset:              "2-15,18-31,34-47",
			expectedEligible:     true,
			expectedAffinityCore: 24,
		},
		{
			name:             "no eligible core small isolated set",
			cpuset:           "60-63",
			isolset:          "0-1",
			expectedEligible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			affinityCore, err := getEligibleCPU(tt.cpuset, tt.isolset)
			eligible := affinityCore >= 0
			if err != nil && !tt.expectedErr {
				t.Fatalf("unexpected error: %s", err)
			} else if err == nil && tt.expectedErr {
				t.Fatalf("unexpected success")
			} else if tt.expectedEligible && !eligible {
				t.Fatalf("was expecting eligible core but no eligible core returned")
			} else if !tt.expectedEligible && eligible {
				t.Fatalf("was not expecting eligible core but got eligible core")
			} else if tt.expectedEligible && tt.expectedAffinityCore != affinityCore {
				t.Fatalf("expected affinity core %d: got %d instead", tt.expectedAffinityCore, affinityCore)
			}
		})
	}
}
