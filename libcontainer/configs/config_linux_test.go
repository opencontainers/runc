package configs

import (
	"testing"
)

var HookNameList = []HookName{Prestart, CreateRuntime, CreateContainer, StartContainer, Poststart, Poststop}

func TestRemoveNamespace(t *testing.T) {
	ns := Namespaces{
		{Type: NEWNET},
	}
	if !ns.Remove(NEWNET) {
		t.Fatal("NEWNET was not removed")
	}
	if len(ns) != 0 {
		t.Fatalf("namespaces should have 0 items but reports %d", len(ns))
	}
}

func TestHostRootUIDNoUSERNS(t *testing.T) {
	config := &Config{
		Namespaces: Namespaces{},
	}
	uid, err := config.HostRootUID()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 0 {
		t.Fatalf("expected uid 0 with no USERNS but received %d", uid)
	}
}

func TestHostRootUIDWithUSERNS(t *testing.T) {
	config := &Config{
		Namespaces: Namespaces{{Type: NEWUSER}},
		UIDMappings: []IDMap{
			{
				ContainerID: 0,
				HostID:      1000,
				Size:        1,
			},
		},
	}
	uid, err := config.HostRootUID()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1000 {
		t.Fatalf("expected uid 1000 with no USERNS but received %d", uid)
	}
}

func TestHostRootGIDNoUSERNS(t *testing.T) {
	config := &Config{
		Namespaces: Namespaces{},
	}
	uid, err := config.HostRootGID()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 0 {
		t.Fatalf("expected gid 0 with no USERNS but received %d", uid)
	}
}

func TestHostRootGIDWithUSERNS(t *testing.T) {
	config := &Config{
		Namespaces: Namespaces{{Type: NEWUSER}},
		GIDMappings: []IDMap{
			{
				ContainerID: 0,
				HostID:      1000,
				Size:        1,
			},
		},
	}
	uid, err := config.HostRootGID()
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1000 {
		t.Fatalf("expected gid 1000 with no USERNS but received %d", uid)
	}
}
