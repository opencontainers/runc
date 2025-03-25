package unixutils

import (
	"errors"

	"golang.org/x/sys/unix"
)

// RetryOnEINTR takes a function that returns an error and calls it until it the error returned is
// not EINTR.
func RetryOnEINTR(fn func() error) error {
	var err error
	for {
		err = fn()
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

// RetryOnEINTR2 is like RetryOnEINTR, but it returns 2 values.
func RetryOnEINTR2[T any](fn func() (T, error)) (val T, err error) {
	for {
		val, err = fn()
		if !errors.Is(err, unix.EINTR) {
			return val, err
		}
	}
}
