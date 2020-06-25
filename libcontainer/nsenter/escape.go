package nsenter

// This file is part of escape_json_string unit test. It would be a part
// of escape_test.go if Go would allow cgo to be used in _test.go files.

// #include <stdlib.h>
// #include "escape.h"
import "C"

import (
	"testing"
	"unsafe"
)

func testEscapeJsonString(t *testing.T, input, want string) {
	in := C.CString(input)
	out := C.escape_json_string(in)
	got := C.GoString(out)
	C.free(unsafe.Pointer(out))
	t.Logf("input: %q, output: %q", input, got)
	if got != want {
		t.Errorf("Failed on input: %q, want %q, got %q", input, want, got)
	}
}
