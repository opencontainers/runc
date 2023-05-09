package libcontainer

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// mountError holds an error from a failed mount or unmount operation.
type mountError struct {
	op     string
	source string
	srcFD  string
	target string
	dstFD  string
	flags  uintptr
	data   string
	err    error
}

// Error provides a string error representation.
func (e *mountError) Error() string {
	out := e.op + " "

	if e.source != "" {
		out += "src=" + e.source + ", "
		if e.srcFD != "" {
			out += "srcFD=" + e.srcFD + ", "
		}
	}
	out += "dst=" + e.target
	if e.dstFD != "" {
		out += ", dstFD=" + e.dstFD
	}

	if e.flags != uintptr(0) {
		out += ", flags=0x" + strconv.FormatUint(uint64(e.flags), 16)
	}
	if e.data != "" {
		out += ", data=" + e.data
	}

	out += ": " + e.err.Error()
	return out
}

// Unwrap returns the underlying error.
// This is a convention used by Go 1.13+ standard library.
func (e *mountError) Unwrap() error {
	return e.err
}

// mount is a simple unix.Mount wrapper, returning an error with more context
// in case it failed.
func mount(source, target, fstype string, flags uintptr, data string) error {
	return mountViaFDs(source, "", target, "", fstype, flags, data)
}

// mountViaFDs is a unix.Mount wrapper which uses srcFD instead of source,
// and dstFD instead of target, unless those are empty. The *FD arguments,
// if non-empty, are expected to be in the form of a path to an opened file
// descriptor on procfs (i.e. "/proc/self/fd/NN").
//
// If case an FD is used instead of a source or a target path, the
// corresponding path is only used to add context to an error in case
// the mount operation has failed.
func mountViaFDs(source, srcFD, target, dstFD, fstype string, flags uintptr, data string) error {
	src := source
	if srcFD != "" {
		src = srcFD
	}
	dst := target
	if dstFD != "" {
		dst = dstFD
	}
	if err := unix.Mount(src, dst, fstype, flags, data); err != nil {
		return &mountError{
			op:     "mount",
			source: source,
			srcFD:  srcFD,
			target: target,
			dstFD:  dstFD,
			flags:  flags,
			data:   data,
			err:    err,
		}
	}
	return nil
}

// unmount is a simple unix.Unmount wrapper.
func unmount(target string, flags int) error {
	err := unix.Unmount(target, flags)
	if err != nil {
		return &mountError{
			op:     "unmount",
			target: target,
			flags:  uintptr(flags),
			err:    err,
		}
	}
	return nil
}
