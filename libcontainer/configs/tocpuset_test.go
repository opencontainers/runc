package configs

import (
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

func TestToCPUSet(t *testing.T) {
	set := func(cpus ...int) unix.CPUSetDynamic {
		maxCPU := 0
		for _, cpu := range cpus {
			if cpu > maxCPU {
				maxCPU = cpu
			}
		}
		r := unix.NewCPUSet(maxCPU + 1)
		for _, cpu := range cpus {
			r.Set(cpu)
		}
		return r
	}

	// trim removes trailing all-zero masks so that sets representing the
	// same CPUs compare equal regardless of how they were allocated.
	trim := func(s unix.CPUSetDynamic) unix.CPUSetDynamic {
		for len(s) > 0 && s[len(s)-1] == 0 {
			s = s[:len(s)-1]
		}
		return s
	}

	testCases := []struct {
		in    string
		out   unix.CPUSetDynamic
		isErr bool
	}{
		{in: ""}, // Empty means unset.

		// Valid cases.
		{in: "0", out: set(0)},
		{in: "1", out: set(1)},
		{in: "0-1", out: set(0, 1)},
		{in: "0,1", out: set(0, 1)},
		{in: ",0,1,", out: set(0, 1)},
		{in: "0-3", out: set(0, 1, 2, 3)},
		{in: "0,1,2-3", out: set(0, 1, 2, 3)},
		{in: "4-7", out: set(4, 5, 6, 7)},
		{in: "0-7", out: set(0, 1, 2, 3, 4, 5, 6, 7)},
		{in: "0-15", out: set(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15)},
		{in: "16", out: set(16)},
		// Extra whitespace in between ranges are OK.
		{in: "1, 2, 1-2", out: set(1, 2)},
		{in: "    , 1   , 3  ,  5-7,    ", out: set(1, 3, 5, 6, 7)},
		// Somewhat large values.
		{in: "0-3,32-33", out: set(0, 1, 2, 3, 32, 33)},
		{in: "127-129, 1", out: set(1, 127, 128, 129)},
		{in: "1023", out: set(1023)},
		// Values larger than what a (non-dynamic) unix.CPUSet can hold.
		{in: "1024", out: set(1024)},
		{in: "8191", out: set(8191)},
		{in: "4096-4098", out: set(4096, 4097, 4098)},
		{in: "0,65536", out: set(0, 65536)},
		// Maximum allowed value.
		{in: "65536", out: set(65536)},

		// Error cases.
		{in: "-", isErr: true},
		{in: "1-", isErr: true},
		{in: "-3", isErr: true},
		{in: ",", isErr: true},
		{in: " ", isErr: true},
		// Bad range (start > end).
		{in: "54-53", isErr: true},
		// Extra spaces inside a range is not OK.
		{in: "1 - 2", isErr: true},
		// Larger than the maximum supported value.
		{in: "65537", isErr: true},
		{in: "0-65537", isErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			out, err := ToCPUSet(tc.in)
			t.Logf("ToCPUSet(%q) = %v (error: %v)", tc.in, out, err)
			// Check the error.
			if tc.isErr {
				if err == nil {
					t.Error("want error, got nil")
				}
				return // No more checks.
			}
			if err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			// Check the value.
			if tc.out == nil {
				if out != nil {
					t.Fatalf("want nil, got %v", out)
				}
				return // No more checks.
			}
			if out == nil {
				t.Fatalf("want %v, got nil", tc.out)
			}
			if !slices.Equal(trim(out), trim(tc.out)) {
				t.Errorf("case %q: want %v, got %v", tc.in, tc.out, out)
			}
		})
	}
}
