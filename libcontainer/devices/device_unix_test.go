// +build !windows

package devices

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func cleanupTest() {
	unixLstat = unix.Lstat
	ioutilReadDir = ioutil.ReadDir
}

func TestDeviceFromPathLstatFailure(t *testing.T) {
	testError := errors.New("test error")

	// Override unix.Lstat to inject error.
	unixLstat = func(path string, stat *unix.Stat_t) error {
		return testError
	}
	defer cleanupTest()

	_, err := DeviceFromPath("", "")
	if err != testError {
		t.Fatalf("Unexpected error %v, expected %v", err, testError)
	}
}

func TestHostDevicesIoutilReadDirFailure(t *testing.T) {
	testError := errors.New("test error")

	// Override ioutil.ReadDir to inject error.
	ioutilReadDir = func(dirname string) ([]os.FileInfo, error) {
		return nil, testError
	}
	defer cleanupTest()

	_, err := HostDevices()
	if err != testError {
		t.Fatalf("Unexpected error %v, expected %v", err, testError)
	}
}

func TestHostDevicesIoutilReadDirDeepFailure(t *testing.T) {
	testError := errors.New("test error")
	called := false

	// Override ioutil.ReadDir to inject error after the first call.
	ioutilReadDir = func(dirname string) ([]os.FileInfo, error) {
		if called {
			return nil, testError
		}
		called = true

		// Provoke a second call.
		fi, err := os.Lstat("/tmp")
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		return []os.FileInfo{fi}, nil
	}
	defer cleanupTest()

	_, err := HostDevices()
	if err != testError {
		t.Fatalf("Unexpected error %v, expected %v", err, testError)
	}
}

func TestHostDevicesAllValid(t *testing.T) {
	devices, err := HostDevices()
	if err != nil {
		t.Fatalf("failed to get host devices: %v", err)
	}

	for _, device := range devices {
		// Devices can't have major number 0.
		if device.Major == 0 {
			t.Errorf("device entry %+v has zero major number", device)
		}
		switch device.Type {
		case BlockDevice, CharDevice:
		case FifoDevice:
			t.Logf("fifo devices shouldn't show up from HostDevices")
			fallthrough
		default:
			t.Errorf("device entry %+v has unexpected type %v", device, device.Type)
		}
	}
}
