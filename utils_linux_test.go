package main

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestCheckProcessRlimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rlimits []specs.POSIXRlimit
		isErr   bool
	}{
		{name: "none"},
		{
			name: "distinct",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_CORE"},
			},
		},
		{
			name: "duplicate",
			rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Soft: 32, Hard: 64},
				{Type: "RLIMIT_NOFILE", Soft: 48, Hard: 64},
			},
			isErr: true,
		},
		{
			name:    "unknown type",
			rlimits: []specs.POSIXRlimit{{Type: "RLIMIT_BOGUS"}},
			isErr:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkProcessRlimits(&specs.Process{Rlimits: tc.rlimits})
			if tc.isErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.isErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
