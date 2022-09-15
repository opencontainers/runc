package fscommon

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const (
	cgroupFile  = "cgroup.file"
	floatValue  = 2048.0
	floatString = "2048"
)

func init() {
	cgroups.TestMode = true
}

func TestGetCgroupParamsInt(t *testing.T) {
	// Setup tempdir.
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, cgroupFile)

	// Success.
	if err := os.WriteFile(tempFile, []byte(floatString), 0o755); err != nil {
		t.Fatal(err)
	}
	value, err := GetCgroupParamUint(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != floatValue {
		t.Fatalf("Expected %d to equal %f", value, floatValue)
	}

	// Success with new line.
	err = os.WriteFile(tempFile, []byte(floatString+"\n"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	value, err = GetCgroupParamUint(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != floatValue {
		t.Fatalf("Expected %d to equal %f", value, floatValue)
	}

	// Success with negative values
	err = os.WriteFile(tempFile, []byte("-12345"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	value, err = GetCgroupParamUint(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != 0 {
		t.Fatalf("Expected %d to equal %d", value, 0)
	}

	// Success with negative values lesser than min int64
	s := strconv.FormatFloat(math.MinInt64, 'f', -1, 64)
	err = os.WriteFile(tempFile, []byte(s), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	value, err = GetCgroupParamUint(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != 0 {
		t.Fatalf("Expected %d to equal %d", value, 0)
	}

	// Not a float.
	err = os.WriteFile(tempFile, []byte("not-a-float"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	_, err = GetCgroupParamUint(tempDir, cgroupFile)
	if err == nil {
		t.Fatal("Expecting error, got none")
	}

	// Unknown file.
	err = os.Remove(tempFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = GetCgroupParamUint(tempDir, cgroupFile)
	if err == nil {
		t.Fatal("Expecting error, got none")
	}
}

func TestParseKeyValueFile(t *testing.T) {
	testCases := []struct {
		Name        string
		FileContent []byte
		FileExist   bool
		Filename    string
		HasErr      bool
		ExpectedErr error
		Expected    map[string]uint64
	}{
		{
			Name:        "Standard memory.events",
			FileContent: []byte("low 0\nhigh 0\nmax 12692218\noom 74039\noom_kill 71934\n"),
			Filename:    "memory.events",
			FileExist:   true,
			HasErr:      false,
			Expected: map[string]uint64{
				"low":      0,
				"high":     0,
				"max":      12692218,
				"oom":      74039,
				"oom_kill": 71934,
			},
		},
		{
			Name:        "File not exists",
			FileExist:   false,
			HasErr:      true,
			ExpectedErr: os.ErrNotExist,
		},
		{
			Name:        "Sample cpu.stat with invalid line",
			FileContent: []byte("usage_usec 27458468773731\nuser_usec 20792829128141\nsystem_usec 6665639645590\n\nval_only\nnon_int xyz\n"),
			FileExist:   true,
			HasErr:      false,
			Expected: map[string]uint64{
				"usage_usec":  27458468773731,
				"user_usec":   20792829128141,
				"system_usec": 6665639645590,
			},
		},
	}

	for _, testCase := range testCases {
		// setup file
		tempDir := t.TempDir()
		if testCase.Filename == "" {
			testCase.Filename = "cgroup.file"
		}

		if testCase.FileExist {
			tempFile := filepath.Join(tempDir, testCase.Filename)

			if err := os.WriteFile(tempFile, testCase.FileContent, 0o755); err != nil {
				t.Fatal(err)
			}
		}

		// get key value
		got, err := ParseKeyValueFile(tempDir, testCase.Filename)
		hasErr := err != nil

		// compare expected
		if testCase.HasErr != hasErr {
			t.Errorf("ParseKeyValueFile returns wrong err: %v for test case: %v", err, testCase.Filename)
		}

		if testCase.ExpectedErr != nil && !errors.Is(err, testCase.ExpectedErr) {
			t.Errorf("ParseKeyValueFile returns wrong err for test case: %v, expected: %v, got: %v",
				testCase.Filename, testCase.Expected, err)
		}

		if !testCase.HasErr {
			if !reflect.DeepEqual(got, testCase.Expected) {
				t.Errorf("ParseKeyValueFile returns wrong result for test case: %v, got: %v, want: %v",
					testCase.Filename, got, testCase.Expected)
			}
		}
	}
}
