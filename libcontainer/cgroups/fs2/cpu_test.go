package fs2

import (
	"testing"
)

func TestConvertCPUQuotaCPUPeriodToCgroupV2Value(t *testing.T) {
	cases := []struct {
		current  string
		quota    int64
		period   uint64
		expected string
	}{
		// quota = 0, period = 0
		{
			current:  "12345 123456",
			quota:    0,
			period:   0,
			expected: "12345 123456",
		},
		{
			current:  "max 123456",
			quota:    0,
			period:   0,
			expected: "max 123456",
		},
		// quota = -1, period = 0
		{
			current:  "12345 123456",
			quota:    -1,
			period:   0,
			expected: "max 123456",
		},
		{
			current:  "max 123456",
			quota:    -1,
			period:   0,
			expected: "max 123456",
		},
		// quota = -1, period > 0
		{
			current:  "12345 123456",
			quota:    -1,
			period:   5000,
			expected: "max 5000",
		},
		{
			current:  "max 123456",
			quota:    -1,
			period:   5000,
			expected: "max 5000",
		},
		// quota > 0, period = 0
		{
			current:  "12345 123456",
			quota:    1000,
			period:   0,
			expected: "1000 123456",
		},
		{
			current:  "max 123456",
			quota:    1000,
			period:   0,
			expected: "1000 123456",
		},
		// quota > 0, period > 0
		{
			current:  "12345 123456",
			quota:    1000,
			period:   5000,
			expected: "1000 5000",
		},
		{
			current:  "max 123456",
			quota:    1000,
			period:   5000,
			expected: "1000 5000",
		},
		// quota = 0, period > 0
		{
			current:  "12345 123456",
			quota:    0,
			period:   5000,
			expected: "12345 5000",
		},
		{
			current:  "max 123456",
			quota:    0,
			period:   5000,
			expected: "max 5000",
		},
		// invalid
		{
			current:  "",
			quota:    1000,
			period:   5000,
			expected: "",
		},
	}
	for i, c := range cases {
		got, err := ConvertCPUQuotaCPUPeriodToCgroupV2Value(c.current, c.quota, c.period)
		if c.expected == "" && err == nil {
			t.Errorf("%d: expected error (c=%+v)", i, c)
			continue
		}
		if c.expected != "" && err != nil {
			t.Errorf("%d: unexpected error: %+v (c=%+v)", i, err, c)
			continue
		}
		if got != c.expected {
			t.Errorf("%d: expected %q, got %q (c=%+v)", i, c.expected, got, c)
		}
	}
}
