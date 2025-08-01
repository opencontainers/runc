package intelrdt

import (
	"testing"
)

func TestRemoveEmptyLines(t *testing.T) {
	data := []string{"", "foo", "", "", "bar", ""}
	expectedResult := []string{"foo", "bar"}
	result := removeEmptyLines(data)
	if len(result) != len(expectedResult) {
		t.Error("result lengths don't match")
	}
	if result[0] != expectedResult[0] || result[1] != expectedResult[1] {
		t.Error("result contents don't match")
	}
}

func TestSchemataLineComparison(t *testing.T) {
	for i, tc := range []struct {
		a, b  string
		match bool
	}{
		// Identity case for MB.
		{"MB:0=5000;1=7000", "MB:0=5000;1=7000", true},
		// Different order.
		{"MB:1=7000;0=5000", "MB:0=5000;1=7000", true},
		// Identity case for L3.
		{"L3:0=f0;1=f", "L3:0=f0;1=f", true},
		// L3 schemata with many tokens.
		{"L3:0=f0;1=f;2=e;3=d;4=a", "L3:0=f0;1=f;2=e;3=d;4=a", true},
		// L3 schemata with spaces.
		{"L3: 0= f0; 1 =   f", "L3:  0 = f0 ;  1 =f", true},
		// L3 schemata with leading zeros.
		{"L3:0=f0;1=000f;2=0000", "L3:0=00f0;1=f;2=0", true},
		// L3 schemata with spaces and leading zeros.
		{"L3: 0= f0; 1 =   000f;", "L3:  0 = 00f0 ;  1 =f;", true},
		// L3 schemata with different order.
		{"L3:0=f0;1=f", "L3:  1=f; 0 = 00f0 ;", true},

		// Various parsing failures. If not the parsing failure, the same strings should match.
		{"L3:  =f; 0 = 00f0 ;", "L3:  =f; 0 = 00f0 ;", false},
		{"L3: 1=f; 0 = bar ;", "L3: 1=f; 0 = bar ;", false},
		{"L3: 1=f; 0 = ;", "L3: 1=f; 0 = ;", false},
		{"L3: 1=f=0;", "L3: 1=f=0;", false},
		{"MB:0=5000;1=;", "MB:0=5000;1=;", false},
		{"MB:=5000;1=7000;", "MB:=5000;1=7000;", false},
		{"MB:5000;1=7000", "MB:5000;1=7000", false},
		{"MB:5000=1=7000", "MB:5000=1=7000", false},

		// Try different schemas (which should not match).
		{"L3:0=f0;1=f", "L3:0=f0;1=d", false},
		{"MB:1=7000;0=5000", "MB:1=5000;0=5000", false},

		// Test multi-line schemata.
		{"\nMB:0=70;1 = 20\nL3:1=   f ;0 = 00f0;", "L3:1=000f;0= f0\nMB: 0=  70;1=20\n\n", true},
		{"\nMB:0=70;1 = 20\nL3:1=   f ;0 = 00f0;", "L3:1=000e;0= f0\nMB: 0=  70;1=20\n\n", false},
		{"\nMB:0=70;1 = 20\nL3:1=   f ;0 = 00f0;", "L3:1=000f;0= f0\nMB: 0=  50;1=20\n\n", false},

		// Test comparing different token types.
		{"MB:1=7000;0=5000", "L3:0=5000;1=7000", false},

		// Test L3DATA and L3CODE.
		{"L3DATA:0=e;1=f\nL3CODE:0=f;1=e", "L3DATA:0=e;1=f\nL3CODE:0=f;1=e", true},
		{"L3DATA:0=e;1=f\nL3CODE:0=f;1=e", "L3DATA:0=e;1=f\nL3CODE:0=a;1=e", false},
		{"L3DATA:0=e;1=3\nL3CODE:0=f;1=e", "L3DATA:0=e;1=f\nL3CODE:0=f;1=e", false},

		// Test that unknown lines are ignored.
		{"L3:0=f0;1=f", "L2:0=5000;1=7000\nL3:0=f0;1=f", true},

		// Test empty lines (which are allowed if they have the same type).
		{"L3: ; ;;", "L3: ;", true},
		{"MB: ; ;;", "MB: ;", true},
		{"L3: ; ;;", "MB: ;", false},
		{" \n;;", "", true},
		{" ", " ", true},
	} {
		err := checkSchemataMatch(tc.a, tc.b)
		if tc.match {
			if err != nil {
				t.Errorf("case %d: schematas '%s' and '%s' should have matched: %s", i, tc.a, tc.b, err)
			}
		} else {
			if err == nil {
				t.Errorf("case %d: schematas '%s' and '%s' should not have matched", i, tc.a, tc.b)
			}
		}
	}
}
