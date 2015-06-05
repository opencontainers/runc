package stacktrace

import "testing"

func TestParsePackageName(t *testing.T) {
	var (
		name             = "github.com/opencontainers/libcontainer/stacktrace.captureFunc"
		expectedPackage  = "github.com/opencontainers/libcontainer/stacktrace"
		expectedFunction = "captureFunc"
	)

	pack, funcName := parseFunctionName(name)
	if pack != expectedPackage {
		t.Fatalf("expected package %q but received %q", expectedPackage, pack)
	}

	if funcName != expectedFunction {
		t.Fatalf("expected function %q but received %q", expectedFunction, funcName)
	}
}
