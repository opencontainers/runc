package libcontainer

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/moby/sys/user"
	"github.com/sirupsen/logrus"
)

// prepareEnv processes a list of environment variables, preparing it
// for direct consumption by unix.Exec. In particular, it:
//   - validates each variable is in the NAME=VALUE format and
//     contains no \0 (nil) bytes;
//   - removes any duplicates (keeping only the last value for each key)
//   - sets PATH for the current process, if found in the list;
//   - adds HOME to returned environment, if not found in the list,
//     or the value is empty.
//
// Returns the prepared environment.
func prepareEnv(env []string, uid int) ([]string, error) {
	if env == nil {
		return nil, nil
	}
	var homeIsSet bool

	// Deduplication code based on dedupEnv from Go 1.22 os/exec.

	// Construct the output in reverse order, to preserve the
	// last occurrence of each key.
	out := make([]string, 0, len(env))
	saw := make(map[string]bool, len(env))
	for n := len(env); n > 0; n-- {
		kv := env[n-1]
		i := strings.IndexByte(kv, '=')
		if i == -1 {
			return nil, errors.New("invalid environment variable: missing '='")
		}
		if i == 0 {
			return nil, errors.New("invalid environment variable: name cannot be empty")
		}
		key := kv[:i]
		val := kv[i+1:]
		if saw[key] { // Duplicate.
			continue
		}
		saw[key] = true
		if strings.IndexByte(kv, 0) >= 0 {
			return nil, fmt.Errorf("invalid environment variable %q: contains nul byte (\\x00)", key)
		}
		if key == "PATH" {
			// Needs to be set as it is used for binary lookup.
			if err := os.Setenv("PATH", val); err != nil {
				return nil, err
			}
		}
		if key == "HOME" {
			if val != "" {
				homeIsSet = true
			} else {
				// Don't add empty HOME to the environment, we will override it later.
				continue
			}
		}
		out = append(out, kv)
	}
	// Restore the original order.
	slices.Reverse(out)

	// If HOME is not found in env, get it from container's /etc/passwd and add.
	if !homeIsSet {
		home, err := getUserHome(uid)
		if err != nil {
			// For backward compatibility, don't return an error, but merely log it.
			logrus.WithError(err).Debugf("HOME not set in process.env, and getting UID %d homedir failed", uid)
		}

		out = append(out, "HOME="+home)
	}

	return out, nil
}

func getUserHome(uid int) (string, error) {
	const defaultHome = "/" // Default value, return this with any error.

	u, err := user.LookupUid(uid)
	if err != nil {
		// ErrNoPasswdEntries is kinda expected as any UID can be specified.
		if errors.Is(err, user.ErrNoPasswdEntries) {
			err = nil
		}
		return defaultHome, err
	}

	return u.Home, nil
}
