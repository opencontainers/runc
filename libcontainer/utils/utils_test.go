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

func TestCleanPath(t *testing.T) {
	path := CleanPath("")
	if path != "" {
		t.Errorf("expected to receive empty string and received %s", path)
	}

	path = CleanPath("rootfs")
	if path != "rootfs" {
		t.Errorf("expected to receive 'rootfs' and received %s", path)
	}

	path = CleanPath("../../../var")
	if path != "var" {
		t.Errorf("expected to receive 'var' and received %s", path)
	}

	path = CleanPath("/../../../var")
	if path != "/var" {
		t.Errorf("expected to receive '/var' and received %s", path)
	}

	path = CleanPath("/foo/bar/")
	if path != "/foo/bar" {
		t.Errorf("expected to receive '/foo/bar' and received %s", path)
	}

	path = CleanPath("/foo/bar/../")
	if path != "/foo" {
		t.Errorf("expected to receive '/foo' and received %s", path)
	}
}

func TestStripRoot(t *testing.T) {
	for _, test := range []struct {
		root, path, out string
	}{
		// Works with multiple components.
		{"/a/b", "/a/b/c", "/c"},
		{"/hello/world", "/hello/world/the/quick-brown/fox", "/the/quick-brown/fox"},
		// '/' must be a no-op.
		{"/", "/a/b/c", "/a/b/c"},
		// Must be the correct order.
		{"/a/b", "/a/c/b", "/a/c/b"},
		// Must be at start.
		{"/abc/def", "/foo/abc/def/bar", "/foo/abc/def/bar"},
		// Must be a lexical parent.
		{"/foo/bar", "/foo/barSAMECOMPONENT", "/foo/barSAMECOMPONENT"},
		// Must only strip the root once.
		{"/foo/bar", "/foo/bar/foo/bar/baz", "/foo/bar/baz"},
		// Deal with .. in a fairly sane way.
		{"/foo/bar", "/foo/bar/../baz", "/foo/baz"},
		{"/foo/bar", "../../../../../../foo/bar/baz", "/baz"},
		{"/foo/bar", "/../../../../../../foo/bar/baz", "/baz"},
		{"/foo/bar/../baz", "/foo/baz/bar", "/bar"},
		{"/foo/bar/../baz", "/foo/baz/../bar/../baz/./foo", "/foo"},
		// All paths are made absolute before stripping.
		{"foo/bar", "/foo/bar/baz/bee", "/baz/bee"},
		{"/foo/bar", "foo/bar/baz/beef", "/baz/beef"},
		{"foo/bar", "foo/bar/baz/beets", "/baz/beets"},
	} {
		got := stripRoot(test.root, test.path)
		if got != test.out {
			t.Errorf("stripRoot(%q, %q) -- got %q, expected %q", test.root, test.path, got, test.out)
		}
	}
}
