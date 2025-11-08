// Package utils provides general helper utilities used in libcontainer.
package utils

import (
	"encoding/json"
	"io"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/pathrs"
)

const (
	exitSignalOffset = 128
)

// ExitStatus returns the correct exit status for a process based on if it
// was signaled or exited cleanly
func ExitStatus(status unix.WaitStatus) int {
	if status.Signaled() {
		return exitSignalOffset + int(status.Signal())
	}
	return status.ExitStatus()
}

// WriteJSON writes the provided struct v to w using standard json marshaling
// without a trailing newline. This is used instead of json.Encoder because
// there might be a problem in json decoder in some cases, see:
// https://github.com/docker/docker/issues/14203#issuecomment-174177790
func WriteJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// SearchLabels searches through a list of key=value pairs for a given key,
// returning its value, and the binary flag telling whether the key exist.
func SearchLabels(labels []string, key string) (string, bool) {
	key += "="
	for _, s := range labels {
		if val, ok := strings.CutPrefix(s, key); ok {
			return val, true
		}
	}
	return "", false
}

// Annotations returns the bundle path and user defined annotations from the
// libcontainer state.  We need to remove the bundle because that is a label
// added by libcontainer.
func Annotations(labels []string) (bundle string, userAnnotations map[string]string) {
	userAnnotations = make(map[string]string)
	for _, l := range labels {
		name, value, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		if name == "bundle" {
			bundle = value
		} else {
			userAnnotations[name] = value
		}
	}
	return bundle, userAnnotations
}

// CleanPath makes a path safe for use with filepath.Join. This is done by not
// only cleaning the path, but also (if the path is relative) adding a leading
// '/' and cleaning it (then removing the leading '/'). This ensures that a
// path resulting from prepending another path will always resolve to lexically
// be a subdirectory of the prefixed path. This is all done lexically, so paths
// that include symlinks won't be safe as a result of using CleanPath.
//
// Deprecated: This function has been moved to internal/pathrs and this wrapper
// will be removed in runc 1.5.
var CleanPath = pathrs.LexicallyCleanPath

// StripRoot returns the passed path, stripping the root path if it was
// (lexicially) inside it. Note that both passed paths will always be treated
// as absolute, and the returned path will also always be absolute. In
// addition, the paths are cleaned before stripping the root.
//
// Deprecated: This function has been moved to internal/pathrs and this wrapper
// will be removed in runc 1.5.
var StripRoot = pathrs.LexicallyStripRoot
