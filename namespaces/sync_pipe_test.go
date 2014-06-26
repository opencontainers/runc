package namespaces

import (
	"fmt"
	"testing"
)

func TestSendErrorFromChild(t *testing.T) {
	pipe, err := NewSyncPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pipe.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	expected := "something bad happened"

	pipe.ReportChildError(fmt.Errorf(expected))

	childError := pipe.ReadFromChild()
	if childError == nil {
		t.Fatal("expected an error to be returned but did not receive anything")
	}

	if childError.Error() != expected {
		t.Fatalf("expected %q but received error message %q", expected, childError.Error())
	}
}

func TestSendPayloadToChild(t *testing.T) {
	pipe, err := NewSyncPipe()
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := pipe.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	expected := "libcontainer"

	if err := pipe.SendToChild(map[string]string{"name": expected}); err != nil {
		t.Fatal(err)
	}

	payload, err := pipe.ReadFromParent()
	if err != nil {
		t.Fatal(err)
	}

	if len(payload) != 1 {
		t.Fatalf("expected to only have one value in the payload but received %d", len(payload))
	}

	if name := payload["name"]; name != expected {
		t.Fatalf("expected name %q but received %q", expected, name)
	}
}
