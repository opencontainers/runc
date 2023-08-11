//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package user

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/opencontainers/runc/libcontainer/system"

	"golang.org/x/sys/unix"
)

// Unix-specific path to the passwd and group formatted files.
const (
	unixPasswdPath = "/etc/passwd"
	unixGroupPath  = "/etc/group"
)

var (
	entOnce   sync.Once
	getentCmd string
)

// LookupUser looks up a user by their username in /etc/passwd. If the user
// cannot be found (or there is no /etc/passwd file on the filesystem), then
// LookupUser returns an error.
func LookupUser(name string) (User, error) {
	return getentUser(name, func(u User) bool {
		return u.Name == name
	})
}

// LookupUid looks up a user by their user id in /etc/passwd. If the user cannot
// be found (or there is no /etc/passwd file on the filesystem), then LookupId
// returns an error.
func LookupUid(uid int) (User, error) {
	return getentUser(fmt.Sprint(uid), func(u User) bool {
		return u.Uid == uid
	})
}

// LookupGroup looks up a group by its name in /etc/group. If the group cannot
// be found (or there is no /etc/group file on the filesystem), then LookupGroup
// returns an error.
func LookupGroup(name string) (Group, error) {
	return getentGroup(fmt.Sprintf("%s %s", "group", name), func(g Group) bool {
		return g.Name == name
	})
}

// LookupGid looks up a group by its group id in /etc/group. If the group cannot
// be found (or there is no /etc/group file on the filesystem), then LookupGid
// returns an error.
func LookupGid(gid int) (Group, error) {
	return getentGroup(fmt.Sprintf("%s %d", "group", gid), func(g Group) bool {
		return g.Gid == gid
	})
}

func getentUser(value string, filter func(u User) bool) (User, error) {
	passwd, err := callGetent("passwd", value)
	if err != nil {
		return User{}, err
	}

	users, err := ParsePasswdFilter(passwd, filter)
	if err != nil {
		return User{}, err
	}

	if len(users) == 0 {
		return User{}, fmt.Errorf("getent failed to find passwd entry for %q", value)
	}

	return users[0], nil
}

func getentGroup(value string, filter func(g Group) bool) (Group, error) {
	group, err := callGetent("group", value)
	if err != nil {
		return Group{}, err
	}

	groups, err := ParseGroupFilter(group, filter)
	if err != nil {
		return Group{}, err
	}

	if len(groups) == 0 {
		return Group{}, fmt.Errorf("getent failed to find groups entry for %q", value)
	}

	return groups[0], nil
}

func callGetent(database string, key string) (io.Reader, error) {
	entOnce.Do(func() { getentCmd, _ = resolveBinary("getent") })
	// if no `getent` command on host, can't do anything else
	if getentCmd == "" {
		return nil, fmt.Errorf("unable to find getent command")
	}
	out, err := execCmd(getentCmd, database, key)
	if err != nil {
		exitCode, errC := system.GetExitCode(err)
		if errC != nil {
			return nil, err
		}
		switch exitCode {
		case 1:
			return nil, fmt.Errorf("getent reported invalid parameters/database unknown")
		case 2:
			return nil, fmt.Errorf("getent unable to find entry %q in %s database", key, database)
		case 3:
			return nil, fmt.Errorf("getent database doesn't support enumeration")
		default:
			return nil, err
		}

	}
	return bytes.NewReader(out), nil
}

func resolveBinary(binname string) (string, error) {
	binaryPath, err := exec.LookPath(binname)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return "", err
	}
	// only return no error if the final resolved binary basename
	// matches what was searched for
	if filepath.Base(resolvedPath) == binname {
		return resolvedPath, nil
	}
	return "", fmt.Errorf("Binary %q does not resolve to a binary of that name in $PATH (%q)", binname, resolvedPath)
}

func execCmd(cmd string, arg ...string) ([]byte, error) {
	execCmd := exec.Command(cmd, arg...)
	return execCmd.CombinedOutput()
}

func GetPasswdPath() (string, error) {
	return unixPasswdPath, nil
}

func GetPasswd() (io.ReadCloser, error) {
	return os.Open(unixPasswdPath)
}

func GetGroupPath() (string, error) {
	return unixGroupPath, nil
}

func GetGroup() (io.ReadCloser, error) {
	return os.Open(unixGroupPath)
}

// CurrentUser looks up the current user by their user id in /etc/passwd. If the
// user cannot be found (or there is no /etc/passwd file on the filesystem),
// then CurrentUser returns an error.
func CurrentUser() (User, error) {
	return LookupUid(unix.Getuid())
}

// CurrentGroup looks up the current user's group by their primary group id's
// entry in /etc/passwd. If the group cannot be found (or there is no
// /etc/group file on the filesystem), then CurrentGroup returns an error.
func CurrentGroup() (Group, error) {
	return LookupGid(unix.Getgid())
}

func currentUserSubIDs(fileName string) ([]SubID, error) {
	u, err := CurrentUser()
	if err != nil {
		return nil, err
	}
	filter := func(entry SubID) bool {
		return entry.Name == u.Name || entry.Name == strconv.Itoa(u.Uid)
	}
	return ParseSubIDFileFilter(fileName, filter)
}

func CurrentUserSubUIDs() ([]SubID, error) {
	return currentUserSubIDs("/etc/subuid")
}

func CurrentUserSubGIDs() ([]SubID, error) {
	return currentUserSubIDs("/etc/subgid")
}

func CurrentProcessUIDMap() ([]IDMap, error) {
	return ParseIDMapFile("/proc/self/uid_map")
}

func CurrentProcessGIDMap() ([]IDMap, error) {
	return ParseIDMapFile("/proc/self/gid_map")
}
