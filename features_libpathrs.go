//go:build libpathrs

package main

import (
	"cyphar.com/go-pathrs"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

var PATHRS_MIN_VERSION string

func pathrsVersionString() string {
	info, err := pathrs.LibraryVersion()
	if err != nil {
		panic(err) // should never happen
	}
	return info.VersionString
}

func checkPathrsVersion() bool {
	pathrsVersion := pathrsVersionString()
	if semver.Compare("v"+pathrsVersion, "v"+PATHRS_MIN_VERSION) < 0 {
		logrus.Errorf("pathrs version %s is too old; need >= %s", pathrsVersion, PATHRS_MIN_VERSION)
		return false
	}
	return true
}
