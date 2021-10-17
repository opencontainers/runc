package capabilities

import (
	"io"
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/syndtr/gocapability/capability"
)

func TestNew(t *testing.T) {
	cs := []string{"CAP_CHOWN", "CAP_UNKNOWN", "CAP_UNKNOWN2"}
	conf := configs.Capabilities{
		Bounding:    cs,
		Effective:   cs,
		Inheritable: cs,
		Permitted:   cs,
		Ambient:     cs,
	}

	hook := test.NewGlobal()
	defer hook.Reset()

	logrus.SetOutput(io.Discard)
	caps, err := New(&conf)
	logrus.SetOutput(os.Stderr)

	if err != nil {
		t.Error(err)
	}
	e := hook.AllEntries()
	if len(e) != 1 {
		t.Errorf("expected 1 warning, got %d", len(e))
	}

	expectedLogs := logrus.Entry{
		Level:   logrus.WarnLevel,
		Message: "ignoring unknown or unavailable capabilities: [CAP_UNKNOWN CAP_UNKNOWN2]",
	}

	l := hook.LastEntry()
	if l == nil {
		t.Fatal("expected a warning, but got none")
	}
	if l.Level != expectedLogs.Level {
		t.Errorf("expected %q, got %q", expectedLogs.Level, l.Level)
	}
	if l.Message != expectedLogs.Message {
		t.Errorf("expected %q, got %q", expectedLogs.Message, l.Message)
	}

	if len(caps.caps) != len(capTypes) {
		t.Errorf("expected %d capability types, got %d: %v", len(capTypes), len(caps.caps), caps.caps)
	}

	for _, cType := range capTypes {
		if i := len(caps.caps[cType]); i != 1 {
			t.Errorf("expected 1 capability for %s, got %d: %v", cType, i, caps.caps[cType])
			continue
		}
		if caps.caps[cType][0] != capability.CAP_CHOWN {
			t.Errorf("expected CAP_CHOWN, got %s: ", caps.caps[cType][0])
			continue
		}
	}

	hook.Reset()
}
