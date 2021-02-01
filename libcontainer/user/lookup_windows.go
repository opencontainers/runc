// +build windows

package user

import (
	"os/user"
	"strconv"
)

func lookupUser(username string) (User, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return User{}, err
	}
	return userFromOS(u)
}

func lookupUid(uid int) (User, error) {
	u, err := user.LookupId(strconv.Itoa(uid))
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
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return Group{}, err
	}
	return groupFromOS(g)
}

// userFromOS converts an os/user.(*User) to local User
//
// (This does not include Pass, Shell or Gecos)
func userFromOS(u *user.User) (User, error) {
	newUser := User{
		Name: u.Username,
		Home: u.HomeDir,
	}
	id, err := strconv.Atoi(u.Uid)
	if err != nil {
		return newUser, err
	}
	newUser.Uid = id

	id, err = strconv.Atoi(u.Gid)
	if err != nil {
		return newUser, err
	}
	newUser.Gid = id
	return newUser, nil
}

// groupFromOS converts an os/user.(*Group) to local Group
//
// (This does not include Pass or List)
func groupFromOS(g *user.Group) (Group, error) {
	newGroup := Group{
		Name: g.Name,
	}

	id, err := strconv.Atoi(g.Gid)
	if err != nil {
		return newGroup, err
	}
	newGroup.Gid = id

	return newGroup, nil
}
