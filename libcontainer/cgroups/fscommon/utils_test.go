// +build linux

package fscommon

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
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
	tempDir, err := ioutil.TempDir("", "cgroup_utils_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	tempFile := filepath.Join(tempDir, cgroupFile)

	// Success.
	err = ioutil.WriteFile(tempFile, []byte(floatString), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	value, err := GetCgroupParamUint(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != floatValue {
		t.Fatalf("Expected %d to equal %f", value, floatValue)
	}

	// Success with new line.
	err = ioutil.WriteFile(tempFile, []byte(floatString+"\n"), 0o755)
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
	err = ioutil.WriteFile(tempFile, []byte("-12345"), 0o755)
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
	err = ioutil.WriteFile(tempFile, []byte(s), 0o755)
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
	err = ioutil.WriteFile(tempFile, []byte("not-a-float"), 0o755)
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
