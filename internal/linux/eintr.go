package linux

import (
	"errors"

	"golang.org/x/sys/unix"
)

// retryOnEINTR takes a function that returns an error and calls it
// until the error returned is not EINTR.
func retryOnEINTR(fn func() error) error {
	for {
		err := fn()
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
