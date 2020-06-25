package escapetest

import "testing"

// The actual test function is in escape.go
// so that it can use cgo (import "C").
// This wrapper is here for gotest to find.

func TestEscapeJson(t *testing.T) {
	testEscapeJson(t)
}
