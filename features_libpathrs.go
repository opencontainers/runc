//go:build libpathrs

package main

import (
	"cyphar.com/go-pathrs"
)

func pathrsVersionString() string {
	info, err := pathrs.LibraryVersion()
	if err != nil {
		panic(err) // should never happen
	}
	return info.VersionString
}
