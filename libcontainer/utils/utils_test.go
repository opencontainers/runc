package utils

import "testing"

func TestGenerateName(t *testing.T) {
	name, err := GenerateRandomName("veth", 5)
	if err != nil {
		t.Fatal(err)
	}

	expected := 5 + len("veth")
	if len(name) != expected {
		t.Fatalf("expected name to be %d chars but received %d", expected, len(name))
	}

	name, err = GenerateRandomName("veth", 65)
	if err != nil {
		t.Fatal(err)
	}

	expected = 64 + len("veth")
	if len(name) != expected {
		t.Fatalf("expected name to be %d chars but received %d", expected, len(name))
	}
}

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
