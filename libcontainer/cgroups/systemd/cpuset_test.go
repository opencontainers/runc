package systemd

import (
	"bytes"
	"testing"
)

func TestRangeToBits(t *testing.T) {
	testCases := []struct {
		in    string
		out   []byte
		isErr bool
	}{
		{in: "", isErr: true},
		{in: "0", out: []byte{1}},
		{in: "1", out: []byte{2}},
		{in: "0-1", out: []byte{3}},
		{in: "0,1", out: []byte{3}},
		{in: ",0,1,", out: []byte{3}},
		{in: "0-3", out: []byte{0x0f}},
		{in: "0,1,2-3", out: []byte{0x0f}},
		{in: "4-7", out: []byte{0xf0}},
		{in: "0-7", out: []byte{0xff}},
		{in: "0-15", out: []byte{0xff, 0xff}},
		{in: "16", out: []byte{0, 0, 1}},
		{in: "0-3,32-33", out: []byte{0x0f, 0, 0, 0, 3}},
		// extra spaces and tabs are ok
		{in: "1, 2, 1-2", out: []byte{6}},
		{in: "    , 1   , 3  ,  5-7,	", out: []byte{0xea}},
		// somewhat large values
		{in: "128-130,1", out: []byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7}},

		{in: "-", isErr: true},
		{in: "1-", isErr: true},
		{in: "-3", isErr: true},
		// bad range (start > end)
		{in: "54-53", isErr: true},
		// kernel does not allow extra spaces inside a range
		{in: "1 - 2", isErr: true},
	}

	for _, tc := range testCases {
		out, err := RangeToBits(tc.in)
		if err != nil {
			if !tc.isErr {
				t.Errorf("case %q: unexpected error: %v", tc.in, err)
			}

			continue
		}
		if !bytes.Equal(out, tc.out) {
			t.Errorf("case %q: expected %v, got %v", tc.in, tc.out, out)
		}
	}
}
