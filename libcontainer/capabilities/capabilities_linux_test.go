package capabilities

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/syndtr/gocapability/capability"
)

func TestNew(t *testing.T) {
	cs := []string{"CAP_CHOWN"}
	conf := configs.Capabilities{
		Bounding:    cs,
		Effective:   cs,
		Inheritable: cs,
		Permitted:   cs,
		Ambient:     cs,
	}

	caps, err := New(&conf)
	if err != nil {
		t.Error(err)
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
}
