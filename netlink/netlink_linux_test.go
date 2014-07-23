package netlink

import (
	"net"
	"testing"
)

func TestCreateBridgeWithMac(t *testing.T) {
	name := "testbridge"

	if err := CreateBridge(name, true); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name); err != nil {
		t.Fatal(err)
	}

	// cleanup and tests

	if err := DeleteBridge(name); err != nil {
		t.Fatal(err)
	}

	if _, err := net.InterfaceByName(name); err == nil {
		t.Fatal("expected error getting interface because bridge was deleted")
	}
}
