package utils

import (
	"bytes"
	"os"
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

func TestSystemCPUCores(t *testing.T) {
	t.Run("MultiCore", func(t *testing.T) {
		content := `cpu  5263854 3354 5436110 61362568 22532 728994 208644 796742 0 0
cpu0 720149 490 674391 7571042 4601 103938 42990 109735 0 0
cpu1 595284 389 676327 7761080 2405 77856 25882 95566 0 0
cpu2 727310 508 693322 7562543 3426 102842 28396 105651 0 0
cpu3 601561 304 685817 7751082 2064 80219 17547 92322 0 0
cpu4 713033 504 669261 7586506 2850 105624 39150 106688 0 0
cpu5 595065 328 683341 7761812 2065 77750 17827 91675 0 0
cpu6 720528 458 676161 7595093 3007 101744 21132 103530 0 0
cpu7 590922 371 677486 7773406 2111 79018 15716 91570 0 0
intr 1997458243 37 333 0 0 0 0 3 0 1 0 0 0 183 0 0 90125 0 0 0 0 0 0 0 0 0 458484 0 361539 0 0 0 256 0 1956792 15 0 918260 6 1450411 256422 0 49025 195 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 2640704037
btime 1752714561
processes 5253419
procs_running 2
procs_blocked 0
softirq 580996229 23 230614056 282 2160733 45109 0 40037 116656548 0 231479441
`
		tmpfile, err := os.CreateTemp("", "stat")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())

		if _, err := tmpfile.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := tmpfile.Close(); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(tmpfile.Name())
		if err != nil {
			t.Fatal(err)
		}
		count, err := readSystemCPU(f)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if count != 8 {
			t.Errorf("expected 8 cores, got %d", count)
		}
	})
}
