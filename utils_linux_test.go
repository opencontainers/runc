package main

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestValidateProcessSpecRlimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rlimits []specs.POSIXRlimit
		isErr   bool
	}{
		{
			name: "none",
		},
		{
			name: "distinct",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_CORE", Soft: 0, Hard: 0},
			},
		},
		{
			name: "duplicate with different values",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_NOFILE", Soft: 48, Hard: 64},
			},
			isErr: true,
		},
		{
			name: "duplicate with identical values",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_CORE", Soft: 0, Hard: 0},
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
			},
			isErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := &specs.Process{
				Cwd:     "/",
				Args:    []string{"true"},
				Rlimits: tc.rlimits,
			}
			err := validateProcessSpec(spec)
			if tc.isErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.isErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
