package nsenter

import "testing"

func TestEscapeJsonString(t *testing.T) {
	testCases := []struct {
		input, output string
	}{
		{"", ""},
		{"abcdef", "abcdef"},
		{`\\\\\\`, `\\\\\\\\\\\\`},
		{`with"quote`, `with\"quote`},
		{"\n\r\b\t\f\\", `\n\r\b\t\f\\`},
		{"\007", "\\u0007"},
		{"\017 \020 \037", "\\u000f \\u0010 \\u001f"},
		{"\033", "\\u001b"},
		{"<->", "<->"},
	}

	for _, tc := range testCases {
		testEscapeJsonString(t, tc.input, tc.output)
	}
}
