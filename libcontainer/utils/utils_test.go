package utils

import (
	"bytes"
	"testing"

	"golang.org/x/sys/unix"
)

var labelTest = []struct {
	labels []string
	query  string
	expVal string
	expOk  bool
}{
	{[]string{"bundle=/path/to/bundle"}, "bundle", "/path/to/bundle", true},
	{[]string{"test=a", "test=b"}, "bundle", "", false},
	{[]string{"bundle=a", "test=b", "bundle=c"}, "bundle", "a", true},
	{[]string{"", "test=a", "bundle=b"}, "bundle", "b", true},
	{[]string{"test", "bundle=a"}, "bundle", "a", true},
	{[]string{"test=a", "bundle="}, "bundle", "", true},
}

func TestSearchLabels(t *testing.T) {
	for _, tt := range labelTest {
		v, ok := SearchLabels(tt.labels, tt.query)
		if ok != tt.expOk {
			t.Errorf("expected ok: %v, got %v", tt.expOk, ok)
			continue
		}
		if v != tt.expVal {
			t.Errorf("expected value '%s' for query '%s'; got '%s'", tt.expVal, tt.query, v)
		}
	}
}

func TestExitStatus(t *testing.T) {
	status := unix.WaitStatus(0)
	ex := ExitStatus(status)
	if ex != 0 {
		t.Errorf("expected exit status to equal 0 and received %d", ex)
	}
}

func TestExitStatusSignaled(t *testing.T) {
	status := unix.WaitStatus(2)
	ex := ExitStatus(status)
	if ex != 130 {
		t.Errorf("expected exit status to equal 130 and received %d", ex)
	}
}

func TestWriteJSON(t *testing.T) {
	person := struct {
		Name string
		Age  int
	}{
		Name: "Alice",
		Age:  30,
	}

	var b bytes.Buffer
	err := WriteJSON(&b, person)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"Name":"Alice","Age":30}`
	if b.String() != expected {
		t.Errorf("expected to write %s but was %s", expected, b.String())
	}
}
