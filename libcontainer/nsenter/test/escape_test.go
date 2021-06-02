package escapetest

import "testing"

// The actual test function is in escape.go
// so that it can use cgo (import "C").
// This wrapper is here for gotest to find.

func TestEscapeJSON(t *testing.T) {
	testEscapeJSON(t)
}
