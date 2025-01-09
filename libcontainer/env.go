package libcontainer

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// prepareEnv processes a list of environment variables, preparing it
// for direct consumption by unix.Exec. In particular, it:
//   - validates each variable is in the NAME=VALUE format and
//     contains no \0 (nil) bytes;
//   - removes any duplicates (keeping only the last value for each key)
//   - sets PATH for the current process, if found in the list.
//
// It returns the deduplicated environment, a flag telling whether HOME
// is present in the input, and an error.
func prepareEnv(env []string) ([]string, bool, error) {
	if env == nil {
		return nil, false, nil
	}
	// Deduplication code based on dedupEnv from Go 1.22 os/exec.

	// Construct the output in reverse order, to preserve the
	// last occurrence of each key.
	out := make([]string, 0, len(env))
	saw := make(map[string]bool, len(env))
	for n := len(env); n > 0; n-- {
		kv := env[n-1]
		i := strings.IndexByte(kv, '=')
		if i == -1 {
			return nil, false, errors.New("invalid environment variable: missing '='")
		}
		if i == 0 {
			return nil, false, errors.New("invalid environment variable: name cannot be empty")
		}
		key := kv[:i]
		if saw[key] { // Duplicate.
			continue
		}
		saw[key] = true
		if strings.IndexByte(kv, 0) >= 0 {
			return nil, false, fmt.Errorf("invalid environment variable %q: contains nul byte (\\x00)", key)
		}
		if key == "PATH" {
			// Needs to be set as it is used for binary lookup.
			if err := os.Setenv("PATH", kv[i+1:]); err != nil {
				return nil, false, err
			}
		}
		out = append(out, kv)
	}
	// Restore the original order.
	slices.Reverse(out)

	return out, saw["HOME"], nil
}
