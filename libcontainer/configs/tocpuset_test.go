package configs

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestToCPUSet(t *testing.T) {
	set := func(cpus ...int) *unix.CPUSet {
		r := &unix.CPUSet{}
		for _, cpu := range cpus {
			r.Set(cpu)
		}
		return r
	}

	testCases := []struct {
		in    string
		out   *unix.CPUSet
		isErr bool
	}{
		{in: ""}, // Empty means unset.

		// Valid cases.
		{in: "0", out: &unix.CPUSet{1}},
		{in: "1", out: &unix.CPUSet{2}},
		{in: "0-1", out: &unix.CPUSet{3}},
		{in: "0,1", out: &unix.CPUSet{3}},
		{in: ",0,1,", out: &unix.CPUSet{3}},
		{in: "0-3", out: &unix.CPUSet{0x0f}},
		{in: "0,1,2-3", out: &unix.CPUSet{0x0f}},
		{in: "4-7", out: &unix.CPUSet{0xf0}},
		{in: "0-7", out: &unix.CPUSet{0xff}},
		{in: "0-15", out: &unix.CPUSet{0xffff}},
		{in: "16", out: &unix.CPUSet{0x10000}},
		// Extra whitespace in between ranges are OK.
		{in: "1, 2, 1-2", out: &unix.CPUSet{6}},
		{in: "    , 1   , 3  ,  5-7,    ", out: &unix.CPUSet{0xea}},
		// Somewhat large values. The underlying type in unix.CPUSet
		// can either be uint32 or uint64, so we have to use a helper.
		{in: "0-3,32-33", out: set(0, 1, 2, 3, 32, 33)},
		{in: "127-129, 1", out: set(1, 127, 128, 129)},
		{in: "1023", out: set(1023)},

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
		{in: "1024", isErr: true}, // Too big for unix.CPUSet.
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			out, err := toCPUSet(tc.in)
			t.Logf("toCPUSet(%q) = %v (error: %v)", tc.in, out, err)
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
			if *out != *tc.out {
				t.Errorf("case %q: want %v, got %v", tc.in, tc.out, out)
			}
		})
	}
}
