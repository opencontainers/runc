package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

var labelTest = []struct {
	labels        []string
	query         string
	expectedValue string
}{
	{[]string{"bundle=/path/to/bundle"}, "bundle", "/path/to/bundle"},
	{[]string{"test=a", "test=b"}, "bundle", ""},
	{[]string{"bundle=a", "test=b", "bundle=c"}, "bundle", "a"},
	{[]string{"", "test=a", "bundle=b"}, "bundle", "b"},
	{[]string{"test", "bundle=a"}, "bundle", "a"},
	{[]string{"test=a", "bundle="}, "bundle", ""},
}

func TestSearchLabels(t *testing.T) {
	for _, tt := range labelTest {
		if v := SearchLabels(tt.labels, tt.query); v != tt.expectedValue {
			t.Errorf("expected value '%s' for query '%s'; got '%s'", tt.expectedValue, tt.query, v)
		}
	}
}

func TestResolveRootfs(t *testing.T) {
	dir := "rootfs"
	if err := os.Mkdir(dir, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)

	path, err := ResolveRootfs(dir)
	if err != nil {
		t.Fatal(err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if path != pwd+"/rootfs" {
		t.Errorf("expected rootfs to be abs and was %s", path)
	}
}

func TestResolveRootfsWithSymlink(t *testing.T) {
	dir := "rootfs"
	tmpDir, _ := filepath.EvalSymlinks(os.TempDir())
	if err := os.Symlink(tmpDir, dir); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)

	path, err := ResolveRootfs(dir)
	if err != nil {
		t.Fatal(err)
	}

	if path != tmpDir {
		t.Errorf("expected rootfs to be the real path %s and was %s", path, os.TempDir())
	}
}

func TestResolveRootfsWithNonExistingDir(t *testing.T) {
	_, err := ResolveRootfs("foo")
	if err == nil {
		t.Error("expected error to happen but received nil")
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
