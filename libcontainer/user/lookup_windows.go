// +build windows

package user

import (
	"errors"
	"fmt"
	"io"
	"os/user"
)

func lookupUser(username string) (User, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return User{}, err
	}
	return userFromOS(u)
}

func lookupUid(uid int) (User, error) {
	u, err := user.LookupId(fmt.Sprintf("%d", uid))
	if err != nil {
		return User{}, err
	}
	return userFromOS(u)
}

func lookupGroup(groupname string) (Group, error) {
	g, err := user.LookupGroup(groupname)
	if err != nil {
		return Group{}, err
	}
	return groupFromOS(g)
}

func lookupGid(gid int) (Group, error) {
	g, err := user.LookupGroupId(fmt.Sprintf("%d", gid))
	if err != nil {
		return Group{}, err
	}
	return groupFromOS(g)
}

var notSupported = errors.New("not supported in this build")

func GetPasswdPath() (string, error) {
	return "", notSupported
}

func GetPasswd() (io.ReadCloser, error) {
	return nil, notSupported
}

func GetGroupPath() (string, error) {
	return "", notSupported
}

func GetGroup() (io.ReadCloser, error) {
	return nil, notSupported
}

func CurrentUser() (User, error) {
	return User{}, notSupported
}

func CurrentGroup() (Group, error) {
	return Group{}, notSupported
}
