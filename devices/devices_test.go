package devices

import (
	"errors"
	"os"
	"testing"
)

func TestGetDeviceLstatFailure(t *testing.T) {
	testError := errors.New("test error")

	// Override os.Lstat to inject error.
	osLstat = func(path string) (os.FileInfo, error) {
		return nil, testError
	}

	_, err := GetDevice("", "")
	if err != testError {
		t.Fatalf("Unexpected error %v, expected %v", err, testError)
	}
}
