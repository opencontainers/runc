package exeseal

import "testing"

func TestValidateMode(t *testing.T) {
	cases := []struct {
		desc    string
		in      string
		wantErr bool
	}{
		{"absent (empty string) is valid", "", false},
		{"recognized: independent-data-copy", "independent-data-copy", false},
		{"recognized: ro-shared-page", "ro-shared-page", false},
		{"unrecognized errors", "not-a-real-mode", true},
		{"case-sensitive", "INDEPENDENT-DATA-COPY", true},
		{"no whitespace trimming", " independent-data-copy ", true},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateMode(tc.in)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateMode(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
		})
	}
}
