package label

import "testing"

func TestFormatMountLabel(t *testing.T) {
	expected := `context="foobar"`
	if test := FormatMountLabel("", "foobar"); test != expected {
		t.Fatalf("Format failed. Expected %s, got %s", expected, test)
	}

	expected = `src,context="foobar"`
	if test := FormatMountLabel("src", "foobar"); test != expected {
		t.Fatalf("Format failed. Expected %s, got %s", expected, test)
	}

	expected = `src`
	if test := FormatMountLabel("src", ""); test != expected {
		t.Fatalf("Format failed. Expected %s, got %s", expected, test)
	}

	expected = `fscontext="foobar"`
	if test := FormatMountLabelByType("", "foobar", "fscontext"); test != expected {
		t.Fatalf("Format failed. Expected %s, got %s", expected, test)
	}

	expected = `src,fscontext="foobar"`
	if test := FormatMountLabelByType("src", "foobar", "fscontext"); test != expected {
		t.Fatalf("Format failed. Expected %s, got %s", expected, test)
	}

	expected = `src`
	if test := FormatMountLabelByType("src", "", "rootcontext"); test != expected {
		t.Fatalf("Format failed. Expected %s, got %s", expected, test)
	}
}
