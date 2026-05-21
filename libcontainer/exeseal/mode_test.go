package exeseal

import "testing"

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"independent-data-copy", ModeIndependentDataCopy, false},
		{"ro-shared-page", ModeROSharedPage, false},
		{"", ModeUnset, true},                      // empty must error, not default
		{"auto", ModeUnset, true},                  // not a valid value in this scheme
		{"independent_data_copy", ModeUnset, true}, // underscore typo
		{"INDEPENDENT-DATA-COPY", ModeUnset, true}, // case-sensitive
		{"ro-shared", ModeUnset, true},             // truncated
		{"memfd-clone", ModeUnset, true},           // an earlier name we considered
		{"clone-binary", ModeUnset, true},          // an even earlier name
		{"ro-overlayfs", ModeUnset, true},          // implementation-named alternative
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseMode(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseMode(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestModeStringRoundTrip(t *testing.T) {
	// For every Mode that has a parseable string form, String() should
	// produce something ParseMode accepts (and vice versa).
	for _, m := range []Mode{ModeIndependentDataCopy, ModeROSharedPage} {
		s := m.String()
		got, err := ParseMode(s)
		if err != nil {
			t.Errorf("ParseMode(%q) (from Mode(%d).String()) failed: %v", s, int(m), err)
			continue
		}
		if got != m {
			t.Errorf("round trip: Mode %v -> %q -> Mode %v", m, s, got)
		}
	}
	// ModeUnset deliberately has no parseable string form.
	if s := ModeUnset.String(); s == "independent-data-copy" || s == "ro-shared-page" {
		t.Errorf("ModeUnset.String() = %q collides with a valid annotation value", s)
	}
}
