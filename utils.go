package runc

import (
	"os"
	"path/filepath"
)

// GetDefaultID returns a string to be used as the container id based on the
// current working directory of the nsinit process.  This function panics
// if the cwd is unable to be found based on a system error.
func DefaultID() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Base(cwd)
}

// DefaultImagePath returns the current working directory with checkpoint appended.
func DefaultImagePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, "checkpoint")
}
