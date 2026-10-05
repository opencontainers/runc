package main

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestCheckProcessRlimits(t *testing.T) {
	base := func(rlimits []specs.POSIXRlimit) *specs.Process {
		return &specs.Process{
			Args:    []string{"/bin/true"},
			Cwd:     "/",
			Rlimits: rlimits,
		}
	}

	tests := []struct {
		name    string
		rlimits []specs.POSIXRlimit
		wantErr bool
	}{
		{
			name: "duplicate type with different values",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_NOFILE", Soft: 48, Hard: 64},
			},
			wantErr: true,
		},
		{
			name: "duplicate type with identical values",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
			},
			wantErr: true,
		},
		{
			name: "distinct types",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_CORE", Soft: 0, Hard: 0},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkProcessRlimits(base(test.rlimits))
			if (err != nil) != test.wantErr {
				t.Fatalf("checkProcessRlimits() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
